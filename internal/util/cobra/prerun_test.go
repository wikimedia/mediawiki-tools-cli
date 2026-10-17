package cobrautil

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
)

func TestPersistentPreRunPreservesExecutingCommand(t *testing.T) {
	for depth := 0; depth <= 4; depth++ {
		t.Run(fmt.Sprintf("depth-%d", depth), func(t *testing.T) {
			root := &cobra.Command{Use: "mw"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root.SetContext(ctx)
			var executing *cobra.Command
			rootCalls := 0
			root.PersistentPreRun = func(received *cobra.Command, args []string) {
				rootCalls++
				// Keep the existing parent-hook contract: setup receives its own command.
				if received != root {
					t.Errorf("root hook received %q", received.Name())
				}
				if got := ExecutingCommand(received); got != executing {
					t.Errorf("executing command = %q, want %q", got.Name(), executing.Name())
				}
			}
			parent := root
			path := []string{}
			for i := 0; i < depth; i++ {
				alias := fmt.Sprintf("g%d", i)
				group := &cobra.Command{Use: fmt.Sprintf("group%d", i), Aliases: []string{alias}}
				// Exercise both nested setup hooks and ancestors with no hook at all.
				if i%2 == 0 {
					group.PersistentPreRun = func(received *cobra.Command, args []string) {
						if got := ExecutingCommand(received); got != executing {
							t.Errorf("nested hook lost executing command: %q", got.Name())
						}
						CallAllPersistentPreRun(received, args)
					}
				}
				parent.AddCommand(group)
				parent = group
				path = append(path, alias)
			}
			for _, name := range []string{"first", "second"} {
				leaf := &cobra.Command{Use: name, Aliases: []string{name + "-alias"}, PersistentPreRun: CallAllPersistentPreRun, Run: func(*cobra.Command, []string) {}}
				parent.AddCommand(leaf)
				executing = leaf
				root.SetArgs(append(append([]string{}, path...), name+"-alias"))
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				if root.Context() != ctx {
					t.Fatal("parent hooks changed the original context")
				}
				if ExecutingCommand(root) != root {
					t.Fatal("executing command leaked into the next execution")
				}
			}
			if rootCalls != 2 {
				t.Errorf("root hook called %d times, want 2", rootCalls)
			}
		})
	}
}
