package output

/*
Output order is not guaranteed.
*/
import (
	"encoding/json"
	"fmt"
	"io"
	"text/template"
)

type GoTmpl struct {
	Objects      map[interface{}]interface{}
	Format       string
	TopLevelKeys bool
}

func NewGoTmpl(objects map[interface{}]interface{}, format string, topLevelKeys bool) *GoTmpl {
	return &GoTmpl{
		Objects:      objects,
		Format:       format,
		TopLevelKeys: topLevelKeys,
	}
}

func (m *GoTmpl) Print(writer io.Writer) {
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
	if m.TopLevelKeys {
		_ = tmpl.Execute(writer, m.Objects)
		fmt.Fprintln(writer)
	} else {
		for _, change := range m.Objects {
			_ = tmpl.Execute(writer, change)
			fmt.Fprintln(writer)
		}
	}
}
