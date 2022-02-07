package env

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotenv"
)

// Env command for interacting with a .env file in the given directory
func Env(Short string, directory func() string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: Short,
		RunE:  nil,
	}
	cmd.AddCommand(envDelete(directory))
	cmd.AddCommand(envSet(directory))
	cmd.AddCommand(envGet(directory))
	cmd.AddCommand(envList(directory))
	cmd.AddCommand(envWhere(directory))
	cmd.AddCommand(envClear(directory))
	return cmd
}

func envDelete(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dotenv.FileForDirectory(directory()).Delete(args[0])
		},
	}
}

func envSet(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "set [name] [value]",
		Short: "Set an environment variable",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			dotenv.FileForDirectory(directory()).Set(args[0], args[1])
		},
	}
}

func envGet(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "get [name]",
		Short: "Get an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(dotenv.FileForDirectory(directory()).Get(args[0]))
		},
	}
}

func envList(directory func() string) *cobra.Command {
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

func envWhere(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Output the location of the .env file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(dotenv.FileForDirectory(directory()).Path())
		},
	}
}

func envClear(directory func() string) *cobra.Command {
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
