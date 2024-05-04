package gerrit

import (
	"fmt"
	gogerrit "github.com/andygrunwald/go-gerrit"
	logrus "github.com/sirupsen/logrus"
	cobra "github.com/spf13/cobra"
	"io"
)

// This code is generated by tools/code-gen/main.go. DO NOT EDIT.
func NewGerritAccessCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Short:   "Access Rights Endpoints",
		Use:     "access",
	}
	cmd.AddCommand(NewGerritAccessListCmd())
	return cmd
}
func NewGerritAccessListCmd() *cobra.Command {
	type flags struct {
		project string
	}
	cmdFlags := flags{}
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/access/"
			path = addParamToPath(path, "project", cmdFlags.project)

			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "List Access Rights",
		Use:   "list",
	}
	cmd.Flags().StringVar(&cmdFlags.project, "project", "", "The projects for which the access rights should be returned must be specified as project options. The project can be specified multiple times.")
	cmd.MarkFlagRequired("project")
	return cmd
}
