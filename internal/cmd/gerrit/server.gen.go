package gerrit

import (
	"fmt"
	gogerrit "github.com/andygrunwald/go-gerrit"
	logrus "github.com/sirupsen/logrus"
	cobra "github.com/spf13/cobra"
	"io/ioutil"
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
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/version/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server Version",
		Use:   "version",
	}
	return cmd
}
func NewGerritServerInfoCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/info/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server Info",
		Use:   "info",
	}
	return cmd
}
func NewGerritServerCachesCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/caches/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server Caches",
		Use:   "caches",
	}
	return cmd
}
func NewGerritServerSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/summary/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server summary",
		Use:   "summary",
	}
	return cmd
}
func NewGerritServerCapabilitiesCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/capabilities/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server capabilities",
		Use:   "capabilities",
	}
	return cmd
}
func NewGerritServerTasksCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/tasks/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server tasks",
		Use:   "tasks",
	}
	return cmd
}
func NewGerritServerTopMenusCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/top-menus/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
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
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/preferences/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server user preferences",
		Use:   "user",
	}
	return cmd
}
func NewGerritServerPreferencesDiffCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/preferences.diff/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server diff preferences",
		Use:   "diff",
	}
	return cmd
}
func NewGerritServerPreferencesEditCmd() *cobra.Command {
	cmd := &cobra.Command{

		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/config/server/preferences.edit/"
			client := authenticatedClient()
			response, err := client.Call("GET", path, nil, nil)
			if err != nil {
				logrus.Error(err)
			}
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			body = gogerrit.RemoveMagicPrefixLine(body)
			fmt.Print(string(body))
		},
		Short: "Server edit preferences",
		Use:   "edit",
	}
	return cmd
}