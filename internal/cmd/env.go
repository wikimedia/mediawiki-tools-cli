package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotenv"
)

/*Env top level env command.*/
func Env(Short string) *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: Short,
		RunE:  nil,
	}
}

/*EnvDelete env delete command.*/
func EnvDelete(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dotenv.FileForDirectory(directory()).Delete(args[0])
		},
	}
}

/*EnvSet env set command.*/
func EnvSet(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "set [name] [value]",
		Short: "Set an environment variable",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			dotenv.FileForDirectory(directory()).Set(args[0], args[1])
		},
	}
}

/*EnvGet env get command.*/
func EnvGet(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "get [name]",
		Short: "Get an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(dotenv.FileForDirectory(directory()).Get(args[0]))
		},
	}
}

/*EnvList env list command.*/
func EnvList(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environment variables",
		Run: func(cmd *cobra.Command, args []string) {
			for name, value := range dotenv.FileForDirectory(directory()).List() {
				fmt.Println(name + "=" + value)
			}
		},
	}
}

/*EnvWhere env where command.*/
func EnvWhere(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Output the location of the .env file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(dotenv.FileForDirectory(directory()).Path())
		},
	}
}

/*EnvClear env clear command.*/
func EnvClear(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clears all values from the .env file",
		Run: func(cmd *cobra.Command, args []string) {
			file := dotenv.FileForDirectory(directory())
			for name := range file.List() {
				file.Delete(name)
			}
			fmt.Println("Cleared .env file")
		},
	}
}
