package files

import (
	"fmt"
	"io/ioutil"
)

func packagedFileToBytes(file string) []byte {
	fileReader, err := Open(file)
	if err != nil {
		fmt.Println(err)
	}
	bytes, _ := ioutil.ReadAll(fileReader)
	return bytes
}

/*packagedFileNames.*/
func packagedFileNames() []string {
	keys := make([]string, 0, len(staticFiles))
	for k := range staticFiles {
		keys = append(keys, k)
	}
	return keys
}
