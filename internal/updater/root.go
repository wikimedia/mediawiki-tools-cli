package updater

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

/*CanUpdate will check for updates.*/
func CanUpdate(currentVersion cli.Version, gitSummary string) (bool, string) {
	canUpdate, release := CanUpdateFromGitlab(currentVersion, gitSummary)
	if canUpdate {
		return canUpdate, release
	}
	logrus.Debug("Current version is: " + currentVersion.String() + "\nLatest available is: " + release)

	// When canUpdate is false, we don't have a release to get the version string of
	return canUpdate, "No update available"
}

func DownloadFileResponse(urlStr string) (*http.Response, error) {
	// Check if it's a file:// URL
	if strings.HasPrefix(urlStr, "file://") {
		// Parse the file URL to get the path
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			logrus.Fatal(err)
		}
		filePath := parsedURL.Path

		// Open the file
		file, err := os.Open(filePath)
		if err != nil {
			logrus.Fatal(err)
		}

		// Get file info for content length
		fileInfo, err := file.Stat()
		if err != nil {
			if closeErr := file.Close(); closeErr != nil {
				logrus.Warn(closeErr)
			}
			logrus.Fatal(err)
		}

		// Create a fake HTTP response for the file
		resp := &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        make(http.Header),
			Body:          file,
			ContentLength: fileInfo.Size(),
		}
		return resp, nil
	}

	// Handle regular HTTP(S) URLs
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	resp, err := client.Get(urlStr)
	if err != nil {
		logrus.Fatal(err)
	}

	return resp, nil
}

func IsZipFile(file string) bool {
	f, err := os.Open(file)
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()

	buf := make([]byte, 4)
	_, err = f.Read(buf)
	if err != nil {
		logrus.Fatal(err)
	}

	// Check for the zip file signature
	return string(buf) == "PK\x03\x04"
}

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	dest = filepath.Clean(dest)
	err = os.MkdirAll(dest, 0o755) // #nosec G301 -- extracted release files must remain user-executable.
	if err != nil {
		return err
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path, err := safeZipPath(dest, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
		} else {
			size := f.FileInfo().Size()
			if err := os.MkdirAll(filepath.Dir(path), f.Mode()); err != nil {
				return err
			}
			outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := outFile.Close(); err != nil {
					panic(err)
				}
			}()

			written, err := io.Copy(outFile, io.LimitReader(rc, size+1))
			if err != nil {
				return err
			}
			if written > size {
				return fmt.Errorf("zip entry exceeds declared size: %s", path)
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func safeZipPath(dest, name string) (string, error) {
	path := filepath.Join(dest, name)
	rel, err := filepath.Rel(dest, path)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("illegal file path: %s", path)
	}
	return path, nil
}
