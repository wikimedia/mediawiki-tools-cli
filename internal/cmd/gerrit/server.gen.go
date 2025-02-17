package gerrit

import (
	gogerrit "github.com/andygrunwald/go-gerrit"
	cobra "github.com/spf13/cobra"
	output "gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"io"
)

// This code is generated by tools/code-gen/main.go. DO NOT EDIT.
func NewGerritServerCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Short:   "Server Config Endpoints",
		Use:     "server",
	}
	cmd.AddCommand(NewGerritServerVersionCmd())
	cmd.AddCommand(NewGerritServerInfoCmd())
	cmd.AddCommand(NewGerritServerCachesCmd())
	cmd.AddCommand(NewGerritServerSummaryCmd())
	cmd.AddCommand(NewGerritServerCapabilitiesCmd())
	cmd.AddCommand(NewGerritServerTasksCmd())
	cmd.AddCommand(NewGerritServerTopMenusCmd())
	cmd.AddCommand(NewGerritServerPreferencesCmd())
	return cmd
}
func NewGerritServerVersionCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/version/"
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
		Short: "Server Version",
		Use:   "version",
	}
	return cmd
}
func NewGerritServerInfoCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/info/"
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
		Short: "Server Info",
		Use:   "info",
	}
	return cmd
}
func NewGerritServerCachesCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/caches/"
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
		Short: "Server Caches",
		Use:   "caches",
	}
	return cmd
}
func NewGerritServerSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/summary/"
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
		Short: "Server summary",
		Use:   "summary",
	}
	return cmd
}
func NewGerritServerCapabilitiesCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/capabilities/"
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
		Short: "Server capabilities",
		Use:   "capabilities",
	}
	return cmd
}
func NewGerritServerTasksCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/tasks/"
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
		Short: "Server tasks",
		Use:   "tasks",
	}
	return cmd
}
func NewGerritServerTopMenusCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/top-menus/"
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
		Short: "Server top-menus",
		Use:   "top-menus",
	}
	return cmd
}
func NewGerritServerPreferencesCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Short:   "Server preferences.",
		Use:     "preferences",
	}
	cmd.AddCommand(NewGerritServerPreferencesUserCmd())
	cmd.AddCommand(NewGerritServerPreferencesDiffCmd())
	cmd.AddCommand(NewGerritServerPreferencesEditCmd())
	return cmd
}
func NewGerritServerPreferencesUserCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/preferences/"
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
		Short: "Server user preferences",
		Use:   "user",
	}
	return cmd
}
func NewGerritServerPreferencesDiffCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/preferences.diff/"
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
		Short: "Server diff preferences",
		Use:   "diff",
	}
	return cmd
}
func NewGerritServerPreferencesEditCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/config/server/preferences.edit/"
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
		Short: "Server edit preferences",
		Use:   "edit",
	}
	return cmd
}
