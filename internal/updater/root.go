package updater

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
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

func DownloadFileResponse(url string) (*http.Response, error) {
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	resp, err := client.Get(url)
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

	err = os.MkdirAll(dest, 0o755)
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

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
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
