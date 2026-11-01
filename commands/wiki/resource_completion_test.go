package wiki

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestResourceCompletionIsStatic(t *testing.T) {
	root := NewWikiCmd()
	for _, test := range []struct {
		path         []string
		flag, prefix string
		want         []string
	}{
		{[]string{"ext", "wikibase", "item", "statement", "update"}, "rank", "p", []string{"preferred"}},
		{[]string{"ext", "wikibase", "property", "statement", "add"}, "snaktype", "", []string{"value", "somevalue", "novalue"}},
		{[]string{"page", "diff"}, "output", "", []string{"text", "html", "json", "jq"}},
	} {
		cmd, _, err := root.Find(test.path)
		if err != nil {
			t.Fatal(err)
		}
		complete, ok := cmd.GetFlagCompletionFunc(test.flag)
		if !ok {
			t.Fatal(test.flag)
		}
		got, directive := complete(cmd, nil, test.prefix)
		if !reflect.DeepEqual(got, test.want) || directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("%v: %v %v", test.path, got, directive)
		}
	}
	cmd, _, _ := root.Find([]string{"ext", "wikibase", "item", "get"})
	_, directive := cmd.ValidArgsFunction(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatal(directive)
	}
}
