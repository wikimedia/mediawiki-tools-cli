package demo1

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// ./bin/mw demo
// ./bin/mw demo sub-demo

func NewDemoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "A Demo Command",
		Run: func(cmd *cobra.Command, args []string) {
			//Your logic here
			logrus.Trace("Hello World trace logging (demo)")
			fmt.Println("Hello World (demo)")
		},
	}
	cmd.AddCommand(NewDemoSubCommand())
	return cmd
}

func NewDemoSubCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub-demo",
		Short: "A sub Demo Command",
		Run: func(cmd *cobra.Command, args []string) {
			//Your logic here
			logrus.Trace("Hello World trace logging (sub-demo)")
			fmt.Println("Hello World (sub-demo)")
		},
	}
	return cmd
}
