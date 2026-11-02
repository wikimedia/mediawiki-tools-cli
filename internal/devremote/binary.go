package devremote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	gitlabb "gitlab.wikimedia.org/repos/releng/cli/internal/gitlab"
	"gitlab.wikimedia.org/repos/releng/cli/internal/releaseversion"
)

type binaryResolver struct {
	build                                     cli.VersionAttributes
	platform, architecture, executable, cache string
	lookup                                    func(string, string, string) (*gitlab.ReleaseLink, error)
	client                                    *http.Client
}

func remoteBinary(ctx context.Context, machine string, stdout io.Writer) (*os.File, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	cache, err := localBinaryCache()
	if err != nil {
		return nil, "", err
	}
	r := binaryResolver{
		build: cli.VersionDetails, platform: runtime.GOOS, architecture: runtime.GOARCH, executable: executable,
		cache: cache, lookup: gitlabb.RelengCliReleaseBinaryForPlatform,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
	return r.resolve(ctx, machine, stdout)
}

func exactReleaseTag(build cli.VersionAttributes) string {
	v, err := releaseversion.Parse(build.Version.String())
	if err != nil || !v.Stable() || strings.ContainsAny(v.String(), "-+") || build.GitState == "dirty" {
		return ""
	}
	tag := "v" + v.String()
	if build.GitSummary != "" && build.GitSummary != tag && build.GitSummary != v.String() {
		return ""
	}
	return tag
}

func (r binaryResolver) resolve(ctx context.Context, machine string, stdout io.Writer) (*os.File, string, error) {
	arch, ok := map[string]string{"x86_64": "amd64", "aarch64": "arm64"}[machine]
	if !ok {
		return nil, "", fmt.Errorf("unsupported remote Linux architecture %q", machine)
	}
	tag := exactReleaseTag(r.build)
	if tag == "" {
		if r.platform != "linux" || r.architecture != arch {
			return nil, "", fmt.Errorf("development and nightly builds only copy the running executable: this build is %s/%s, but the VM requires linux/%s; run a matching Linux build or use a published stable release", r.platform, r.architecture, arch)
		}
		return openBinary(r.executable, machine)
	}
	directory := filepath.Join(r.cache, tag, "linux-"+arch)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, "", err
	}
	lock := flock.New(directory + ".lock")
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, "", err
	}
	if !locked {
		return nil, "", errors.New("could not lock remote binary cache")
	}
	defer lock.Unlock()
	filename := filepath.Join(directory, "wm")
	checksum, err := os.ReadFile(filename + ".sha256")
	if err == nil {
		file, digest, err := openBinary(filename, machine)
		if err == nil {
			if digest == string(checksum) {
				return file, digest, nil
			}
			file.Close()
		}
	}
	fmt.Fprintf(stdout, "Downloading %s for linux/%s from GitLab...\n", tag, arch)
	asset, err := r.lookup(tag, "linux", arch)
	if err != nil {
		return nil, "", fmt.Errorf("the exact matching release %s for linux/%s is unavailable; no other version will be used: %w", tag, arch, err)
	}
	// Release checksum files are published alongside each binary asset.
	rawChecksum, err := r.get(ctx, asset.URL+".sha256", 1024)
	if err != nil {
		return nil, "", fmt.Errorf("download checksum for %s: %w", tag, err)
	}
	fields := strings.Fields(string(rawChecksum))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return nil, "", errors.New("invalid release checksum")
	}
	expected, err := hex.DecodeString(fields[0])
	if err != nil {
		return nil, "", errors.New("invalid release checksum")
	}
	temp, err := os.CreateTemp(directory, "download-*")
	if err != nil {
		return nil, "", err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("User-Agent", cli.HTTPUserAgent)
	response, err := r.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download exact release %s: HTTP %d", tag, response.StatusCode)
	}
	hash := sha256.New()
	const limit = 512 << 20
	size, err := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, "", err
	}
	if size > limit {
		return nil, "", errors.New("release binary exceeds size limit")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), hex.EncodeToString(expected)) {
		return nil, "", errors.New("release binary checksum does not match; download discarded")
	}
	if err := temp.Close(); err != nil {
		return nil, "", err
	}
	file, _, err := openBinary(temp.Name(), machine)
	if err != nil {
		return nil, "", err
	}
	file.Close()
	if err := os.Chmod(temp.Name(), 0o700); err != nil {
		return nil, "", err
	}
	// Replace only a cache entry whose validation failed; Windows cannot rename over it.
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return nil, "", err
	}
	if err := os.Rename(temp.Name(), filename); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filename+".sha256", []byte(hex.EncodeToString(expected)), 0o600); err != nil {
		return nil, "", err
	}
	return openBinary(filename, machine)
}

func (r binaryResolver) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", cli.HTTPUserAgent)
	response, err := r.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if int64(len(body)) > limit {
		return nil, errors.New("checksum response too large")
	}
	return body, err
}
