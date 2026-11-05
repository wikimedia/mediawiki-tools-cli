package enterprise

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

type downloadState struct {
	Path string `json:"path"`
	ETag string `json:"etag"`
	Size int64  `json:"size"`
}

// downloadResponse follows storage redirects manually. No storage request ever
// carries the Enterprise Authorization header, including same-host redirects.
func (c *Client) downloadResponse(ctx context.Context, method, path string, offset int64, etag string) (*http.Response, error) {
	resp, err := c.Authorized(ctx, method, c.APIURL+path, nil, "")
	if err != nil {
		return nil, err
	}
	for redirects := 0; resp.StatusCode >= 300 && resp.StatusCode < 400; redirects++ {
		location, err := resp.Location()
		resp.Body.Close()
		if err != nil || redirects >= 5 {
			return nil, fmt.Errorf("invalid Enterprise download redirect")
		}
		if err := storageURL(location); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, location.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("invalid storage request")
		}
		req.Header.Set("User-Agent", cli.HTTPUserAgent)
		req.Header.Set("Accept-Encoding", "identity")
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
			req.Header.Set("If-Range", etag)
		}
		resp, err = c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("Enterprise storage connection failed")
		}
	}
	return resp, nil
}

func storageURL(u *url.URL) error {
	if u.User != nil || u.Fragment != "" {
		return fmt.Errorf("invalid storage redirect")
	}
	// Endpoint validates HTTPS (or loopback HTTP for local tests). Signed query
	// parameters are deliberately excluded from validation and never persisted.
	_, err := secrets.Endpoint((&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String())
	return err
}

func (c *Client) Download(ctx context.Context, path, destination string, resume bool, progress io.Writer) error {
	if destination == "" || destination == "-" {
		return fmt.Errorf("provide --file with a destination filename")
	}
	lock := flock.New(destination + ".lock")
	locked, err := lock.TryLock()
	if err != nil || !locked {
		return fmt.Errorf("download destination is locked by another process")
	}
	defer lock.Unlock()
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	partial, statePath := destination+".part", destination+".part.json"
	var offset int64
	if info, err := os.Lstat(partial); err == nil {
		if !resume {
			return fmt.Errorf("partial file exists; use --resume or select another --file")
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("partial download must be a regular file")
		}
		offset = info.Size()
	} else if !os.IsNotExist(err) {
		return err
	}
	headCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	head, err := c.downloadResponse(headCtx, http.MethodHead, path, 0, "")
	cancel()
	if err != nil {
		return err
	}
	head.Body.Close()
	if head.StatusCode != http.StatusOK {
		return &HTTPError{head.StatusCode}
	}
	state := downloadState{Path: path, ETag: head.Header.Get("ETag"), Size: head.ContentLength}
	if state.Size < 0 {
		return fmt.Errorf("download headers did not include Content-Length")
	}
	if resume && offset > 0 {
		var previous downloadState
		if err := ReadState(statePath, &previous); err != nil {
			return fmt.Errorf("could not read resume metadata: %w", err)
		}
		if state.ETag == "" || strings.HasPrefix(state.ETag, "W/") || previous != state {
			return fmt.Errorf("remote dataset changed or cannot be verified; select another --file or remove the partial file and metadata")
		}
		if offset > state.Size {
			return fmt.Errorf("partial file is larger than the remote dataset")
		}
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if resume && offset > 0 {
		flags = os.O_WRONLY | os.O_APPEND
	}
	// An empty partial file can be reused after a failed initial request.
	if resume && offset == 0 {
		if info, err := os.Lstat(partial); err == nil && info.Mode().IsRegular() {
			flags = os.O_WRONLY
		}
	}
	f, err := os.OpenFile(partial, flags, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := WriteState(statePath, state); err != nil {
		return err
	}
	if offset != state.Size {
		resp, err := c.downloadResponse(ctx, http.MethodGet, path, offset, state.ETag)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if offset > 0 {
			var start, end, total int64
			n, scanErr := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total)
			if resp.StatusCode != http.StatusPartialContent || scanErr != nil || n != 3 || start != offset || total != state.Size || end != state.Size-1 {
				return fmt.Errorf("storage did not honor the resume range; partial file was preserved")
			}
		} else if resp.StatusCode != http.StatusOK {
			return &HTTPError{resp.StatusCode}
		}
		if tag := resp.Header.Get("ETag"); state.ETag != "" && tag != state.ETag {
			return fmt.Errorf("dataset changed before download; partial file was preserved")
		}
		if progress != nil {
			fmt.Fprintf(progress, "Downloading %s (%s bytes remaining)...\n", destination, strconv.FormatInt(state.Size-offset, 10))
		}
		writer := &downloadProgress{file: f, output: progress, name: destination, total: state.Size, done: offset, last: time.Now()}
		written, err := io.Copy(writer, io.LimitReader(resp.Body, state.Size-offset+1))
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("download interrupted; use --resume")
		}
		if written != state.Size-offset {
			return fmt.Errorf("download size mismatch; partial file was preserved")
		}
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(partial, destination); err != nil {
		return err
	}
	if err := os.Remove(statePath); err != nil {
		return err
	}
	if progress != nil {
		fmt.Fprintf(progress, "Saved %s (%d bytes).\n", destination, state.Size)
	}
	return nil
}

type downloadProgress struct {
	file        io.Writer
	output      io.Writer
	name        string
	total, done int64
	last        time.Time
}

func (p *downloadProgress) Write(data []byte) (int, error) {
	n, err := p.file.Write(data)
	p.done += int64(n)
	if p.output != nil && time.Since(p.last) >= 2*time.Second {
		fmt.Fprintf(p.output, "Downloading %s: %d / %d bytes\n", p.name, p.done, p.total)
		p.last = time.Now()
	}
	return n, err
}
