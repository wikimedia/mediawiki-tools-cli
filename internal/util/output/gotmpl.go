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
	Objects interface{}
	Format  string
}

func NewGoTmpl(objects interface{}, format string) *GoTmpl {
	return &GoTmpl{
		Objects: objects,
		Format:  format,
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
	_ = tmpl.Execute(writer, m.Objects)
	fmt.Fprintln(writer)
}
