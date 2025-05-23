package gerrit

import (
	gogerrit "github.com/andygrunwald/go-gerrit"
	cobra "github.com/spf13/cobra"
	output "gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/access/"
			path = addParamToPath(path, "project", cmdFlags.project)

			client := authenticatedClient(cmd.Context())
			response, err := client.Call(cmd.Context(), "GET", path, nil, nil)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			output.NewJSONFromString(string(body), "", false).Print(cmd.OutOrStdout())
			return nil
		},
		Short: "List Access Rights",
		Use:   "list",
	}
	cmd.Flags().StringVar(&cmdFlags.project, "project", "", "The projects for which the access rights should be returned must be specified as project options. The project can be specified multiple times.")
	cmd.MarkFlagRequired("project")
	return cmd
}
