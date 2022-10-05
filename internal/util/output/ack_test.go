package output

import (
	"bytes"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

func TestAck_Print(t *testing.T) {
	type addSection struct {
		name  string
		items []interface{}
	}
	type addItem struct {
		section string
		item    string
	}
	type fields struct {
		AddSections []addSection
		AddItems    []addItem
	}
	tests := []struct {
		name       string
		fields     fields
		wantOutput string
	}{
		{
			name:       "Empty ack",
			fields:     fields{},
			wantOutput: "",
		},
		{
			name: "One section, one value",
			fields: fields{
				AddSections: []addSection{
					{name: "one", items: strings.SplitToInterfaceSplit([]string{"value"})},
				},
			},
			wantOutput: "one:\nvalue\n",
		},
		{
			name: "One section, two values",
			fields: fields{
				AddSections: []addSection{
					{name: "one", items: strings.SplitToInterfaceSplit([]string{"value", "second"})},
				},
			},
			wantOutput: "one:\nvalue\nsecond\n",
		},
		{
			name: "Two section, three values",
			fields: fields{
				AddSections: []addSection{
					{name: "one", items: strings.SplitToInterfaceSplit([]string{"value", "second"})},
					{name: "two", items: strings.SplitToInterfaceSplit([]string{"3rd"})},
				},
			},
			wantOutput: "one:\nvalue\nsecond\n\n\ntwo:\n3rd\n",
		},
		{
			name: "One via AddItem",
			fields: fields{
				AddItems: []addItem{
					{section: "one", item: "val1"},
				},
			},
			wantOutput: "one:\nval1\n",
		},
		{
			name: "Two via AddItem",
			fields: fields{
				AddItems: []addItem{
					{section: "one", item: "val1"},
					{section: "one", item: "val2"},
				},
			},
			wantOutput: "one:\nval1\nval2\n",
		},
		{
			name: "Mixture",
			fields: fields{
				AddSections: []addSection{
					{name: "one", items: strings.SplitToInterfaceSplit([]string{"valx"})},
				},
				AddItems: []addItem{
					{section: "one", item: "val1"},
					{section: "one", item: "val2"},
					{section: "second", item: "new"},
				},
			},
			wantOutput: "one:\nvalx\nval1\nval2\n\n\nsecond:\nnew\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Ack{}
			for _, addSection := range tt.fields.AddSections {
				a.AddSection(addSection.name, addSection.items)
			}
			for _, addItem := range tt.fields.AddItems {
				a.AddItem(addItem.section, addItem.item)
			}
			writer := &bytes.Buffer{}
			a.Print(writer)
			if gotOutput := writer.String(); gotOutput != tt.wantOutput {
				t.Errorf("....Ack.Print()....\n%v\n\n....wanted....\n%v", gotOutput, tt.wantOutput)
			}
		})
	}
}
