package files

import (
	"os"

	"github.com/sirupsen/logrus"
)

func StringToTempFile(s string) string {
	tmpFile, err := os.CreateTemp(os.TempDir(), "mwcli-")
	if err != nil {
		logrus.Fatal("Cannot create temporary file", err)
	}

	logrus.Trace("Created File: " + tmpFile.Name())

	text := []byte(s)
	if _, err = tmpFile.Write(text); err != nil {
		logrus.Fatal("Failed to write to temporary file", err)
	}

	if err := tmpFile.Close(); err != nil {
		logrus.Fatal(err)
	}
	return tmpFile.Name()
}
