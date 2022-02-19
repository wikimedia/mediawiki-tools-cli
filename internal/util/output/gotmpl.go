package output

import (
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

type GoTmpl struct {
	Objects map[interface{}]interface{}
	Format  string
}

func NewGoTmpl(objects map[interface{}]interface{}, format string) *GoTmpl {
	return &GoTmpl{
		Objects: objects,
		Format:  format,
	}
}

func (m *GoTmpl) Print() {
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
		Parse(m.Format))
	for _, change := range m.Objects {
		_ = tmpl.Execute(os.Stdout, change)
		fmt.Println()
	}
}
