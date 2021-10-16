package dirs

import "os"

func EnsureExists(dirPath string) {
	if _, err := os.Stat(dirPath); err != nil {
		mkerr := os.MkdirAll(dirPath, 0o755)
		if mkerr != nil {
			panic(mkerr)
		}
	}
}
