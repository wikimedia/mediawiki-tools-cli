package output

import (
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

func Modern(objects []interface{}, outputFormat string) {
	tmpl := template.Must(template.
		New("").
		Funcs(map[string]interface{}{
			"json": func(v interface{}) (string, error) {
				b, err := json.MarshalIndent(v, "", "  ")
				if err != nil {
					return "", err
				}
				return string(b), nil
			},
		}).
		Parse(outputFormat))
	for _, change := range objects {
		_ = tmpl.Execute(os.Stdout, change)
		fmt.Println()
	}
}
