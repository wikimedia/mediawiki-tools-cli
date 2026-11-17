package dumps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/gofrs/flock"
	"github.com/spf13/cobra"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"golang.org/x/term"
)

func newDownloadCmd(opts *options, ask func(*cobra.Command, string) error) *cobra.Command {
	var file, checksum string
	var resume bool
	cmd := &cobra.Command{
		Use: "download <path>", Short: "Download one dump file without changing its format",
		Long: `Download one exact file from a path shown by list. --file is required.
Downloads are serial, remain compressed and never overwrite an existing
destination. Large history exports have many shards; select each required file.

Interrupted downloads retain .part and .part.json files. --resume requires the
same URL, a strong ETag and matching remote size. Mutable latest aliases can
change; prefer dated paths. A terminal progress bar goes to stderr; redirected
output only reports download start and completion. Ctrl+C preserves partials.

For current/history XML content exports, require a published SHA256SUMS manifest
and verify the selected file against it. The manifest is always read from
Wikimedia, even when downloading from a mirror. For other datasets, --sha256
can supply an expected checksum from a trusted source; without it only transfer
length and HTTP validators are checked, not content integrity.

Third-party downloads require explicit host approval separately from any earlier
speed test. Redirects to additional third-party hosts require their own approval.
No automatic mirror selection, decompression or database import is performed.`,
		Example: cobrautil.NormalizeExample(`
download wikidata/latest-lexemes.json.bz2 --file lexemes.json.bz2
download current/abwiki/2026-11-01/xml/bzip2/abwiki-2026-11-01-p1p57894.xml.bz2 --file abwiki.xml.bz2
download wikidata/latest-lexemes.json.bz2 --file lexemes.json.bz2 --resume
`),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			relative, err := dumpPath(args[0])
			if err != nil {
				return err
			}
			if relative == "" || strings.HasSuffix(relative, "/") {
				return fmt.Errorf("select one file path, not a directory")
			}
			if checksum != "" {
				decoded, err := hex.DecodeString(checksum)
				if err != nil || len(decoded) != sha256.Size {
					return fmt.Errorf("--sha256 must contain exactly 64 hexadecimal characters")
				}
				checksum = strings.ToLower(checksum)
			}
			base, err := mirrorURL(opts.mirror)
			if err != nil {
				return err
			}
			target, err := mirrorTarget(base, relative)
			if err != nil {
				return err
			}
			client := opts.client(cmd, ask)
			defer client.http.CloseIdleConnections()
			return runCancelable(cmd, func(ctx context.Context) error {
				expected, err := client.exportChecksum(ctx, relative, checksum)
				if err != nil {
					return err
				}
				return client.download(ctx, target, file, resume, expected, cmd.ErrOrStderr())
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Destination filename (required; never overwritten)")
	_ = cmd.MarkFlagRequired("file")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume a partial transfer after validating its ETag and size")
	cmd.Flags().StringVar(&checksum, "sha256", "", "Expected SHA-256 from a trusted source (automatic for content exports)")
	return cmd
}

func (c *dumpClient) exportChecksum(ctx context.Context, relative, supplied string) (string, error) {
	if !contentExport(relative) {
		return supplied, nil
	}
	official, _ := url.Parse(officialURL)
	manifestURL := official.JoinPath(path.Dir(relative), "SHA256SUMS")
	data, err := c.metadata(ctx, manifestURL)
	if err != nil {
		return "", fmt.Errorf("content export is not ready or its SHA256SUMS is unavailable: %w", err)
	}
	expected, err := checksumFor(data, path.Base(relative))
	if err != nil {
		return "", err
	}
	if supplied != "" && supplied != expected {
		return "", fmt.Errorf("--sha256 disagrees with the official content export manifest")
	}
	return expected, nil
}

func checksumFor(data []byte, name string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(strings.TrimPrefix(fields[1], "*"), "./") != name {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil || len(sum) != sha256.Size {
			return "", fmt.Errorf("invalid SHA-256 in manifest for %s", name)
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", fmt.Errorf("file %s is not listed in the published SHA256SUMS", name)
}

type downloadState struct {
	URL    string `json:"url"`
	ETag   string `json:"etag"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

func (c *dumpClient) download(ctx context.Context, source *url.URL, destination string, resume bool, checksum string, progress io.Writer) error {
	if destination == "" || destination == "-" {
		return fmt.Errorf("provide --file with a destination filename")
	}
	lock := flock.New(destination + ".lock")
	locked, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !locked {
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
	existing := false
	if info, err := os.Lstat(partial); err == nil {
		if !resume || !info.Mode().IsRegular() {
			return fmt.Errorf("partial file exists; use --resume with a regular file or select another --file")
		}
		existing, offset = true, info.Size()
	} else if !os.IsNotExist(err) {
		return err
	}
	headCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	resp, err := c.request(headCtx, http.MethodHead, source, nil)
	if err != nil {
		cancel()
		return err
	}
	resp.Body.Close()
	cancel()
	if resp.StatusCode != http.StatusOK || resp.ContentLength < 0 {
		return fmt.Errorf("download HEAD requires HTTP 200 and Content-Length (got HTTP %d)", resp.StatusCode)
	}
	state := downloadState{URL: source.String(), ETag: resp.Header.Get("ETag"), Size: resp.ContentLength, SHA256: checksum}
	if existing {
		data, err := os.ReadFile(statePath)
		if err != nil {
			return fmt.Errorf("could not read resume metadata: %w", err)
		}
		var previous downloadState
		if err := json.Unmarshal(data, &previous); err != nil {
			return fmt.Errorf("invalid resume metadata: %w", err)
		}
		if previous != state || offset > state.Size || (offset > 0 && (state.ETag == "" || strings.HasPrefix(state.ETag, "W/"))) {
			return fmt.Errorf("remote file changed or lacks a strong ETag; partial file preserved, select another --file")
		}
	}
	flags := os.O_CREATE | os.O_EXCL | os.O_RDWR
	if existing {
		flags = os.O_RDWR | os.O_APPEND
	}
	file, err := os.OpenFile(partial, flags, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if !existing {
		metadata, err := os.OpenFile(statePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := metadata.Write(data)
		closeErr := metadata.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	var digest hash.Hash
	if checksum != "" {
		digest = sha256.New()
		if _, err := io.Copy(digest, contextReader{ctx: ctx, reader: file}); err != nil {
			return err
		}
	}
	if offset < state.Size {
		headers := make(http.Header)
		if offset > 0 {
			headers.Set("Range", fmt.Sprintf("bytes=%d-", offset))
			headers.Set("If-Range", state.ETag)
		}
		resp, err := c.request(ctx, http.MethodGet, source, headers)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if offset > 0 {
			want := fmt.Sprintf("bytes %d-%d/%d", offset, state.Size-1, state.Size)
			if resp.StatusCode != http.StatusPartialContent || resp.Header.Get("Content-Range") != want {
				return fmt.Errorf("server did not honor the resume range; partial file preserved")
			}
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download GET returned HTTP %d", resp.StatusCode)
		}
		if state.ETag != "" && resp.Header.Get("ETag") != state.ETag {
			return fmt.Errorf("remote ETag changed before download; partial file preserved")
		}
		if _, err := fmt.Fprintf(progress, "Downloading %s: %d / %d bytes\n", destination, offset, state.Size); err != nil {
			return err
		}
		var writer io.Writer = file
		if digest != nil {
			writer = io.MultiWriter(file, digest)
		}
		tracker := newDownloadProgress(writer, progress, offset, state.Size)
		if err := tracker.render(); err != nil {
			return err
		}
		written, copyErr := io.Copy(tracker, io.LimitReader(resp.Body, state.Size-offset+1))
		clearErr := tracker.clear()
		if copyErr != nil {
			return fmt.Errorf("download interrupted; use --resume: %w", errors.Join(copyErr, clearErr))
		}
		if clearErr != nil {
			return clearErr
		}
		if written != state.Size-offset {
			return fmt.Errorf("download size mismatch; partial file preserved")
		}
	}
	if digest != nil && hex.EncodeToString(digest.Sum(nil)) != checksum {
		return fmt.Errorf("SHA-256 mismatch; unverified data retained only in %s", partial)
	}
	if err := file.Sync(); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}
	// Link publishes without replacing a destination created during the download.
	if err := os.Link(partial, destination); err != nil {
		return fmt.Errorf("could not publish download without overwriting: %w", err)
	}
	if err := os.Remove(partial); err != nil {
		return err
	}
	if err := os.Remove(statePath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(progress, "Saved %s (%d bytes).\n", destination, state.Size)
	return err
}

type downloadProgress struct {
	writer      io.Writer
	output      io.Writer
	done, total int64
	last        time.Time
	terminal    bool
	bar         progress.Model
}

func newDownloadProgress(writer, output io.Writer, offset, total int64) *downloadProgress {
	p := &downloadProgress{writer: writer, output: output, done: offset, total: total}
	if file, ok := output.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		p.terminal = true
		width := 40
		if columns, _, err := term.GetSize(int(file.Fd())); err == nil {
			// Leave room for the byte counts, spaces and terminal cursor.
			width = min(width, columns-len(fmt.Sprintf("  %d / %d bytes", total, total))-1)
		}
		p.bar = progress.New(progress.WithDefaultGradient(), progress.WithWidth(max(1, width)))
	}
	return p
}

func (p *downloadProgress) render() error {
	if !p.terminal {
		return nil
	}
	ratio := float64(p.done) / float64(p.total)
	_, err := fmt.Fprintf(p.output, "\r\033[K%s  %d / %d bytes", p.bar.ViewAs(ratio), p.done, p.total)
	p.last = time.Now()
	return err
}

func (p *downloadProgress) clear() error {
	if !p.terminal {
		return nil
	}
	_, err := fmt.Fprint(p.output, "\r\033[K")
	return err
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

func (p *downloadProgress) Write(data []byte) (int, error) {
	n, err := p.writer.Write(data)
	p.done += int64(n)
	if err == nil && time.Since(p.last) >= 100*time.Millisecond {
		err = p.render()
	}
	return n, err
}
