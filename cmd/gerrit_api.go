package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/andygrunwald/go-gerrit"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	gerritAPIMethod   string
	gerritAPIUser     string
	gerritAPIPassword string
)

func NewGerritAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "api",
		Short:   "Gerrits API",
		Example: `api --auth-user Username --auth-password Password accounts/addshore`,
		Long:    `https://gerrit.wikimedia.org/r/Documentation/rest-api.html`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(1)
			}

			instance := "https://gerrit.wikimedia.org/r/"
			client, _ := gerrit.NewClient(instance, nil)
			if gerritAPIUser != "" && gerritAPIPassword != "" {
				logrus.Trace("Using username and password")
				client.Authentication.SetBasicAuth(gerritAPIUser, gerritAPIPassword)
			}

			resp, err := client.Call(gerritAPIMethod, args[0], nil, nil)
			if err != nil {
				logrus.Fatal("Error making request: %s", err)
			}

			if err != nil {
				fmt.Printf("Error: %s\n", err)
			}

			defer resp.Response.Body.Close()

			b, err := io.ReadAll(resp.Response.Body)
			if err != nil {
				log.Fatalln(err)
			}
			b = gerrit.RemoveMagicPrefixLine(b)

			// printing the structure
			fmt.Print(string(b))
		},
	}
	cmd.Flags().StringVarP(&gerritAPIMethod, "method", "X", "GET", "The HTTP method for the request (default \"GET\")")
	cmd.Flags().StringVarP(&gerritAPIUser, "auth-user", "", "", "Gerrit HTTP user")
	cmd.Flags().StringVarP(&gerritAPIPassword, "auth-password", "", "", "Gerrit HTTP password")
	return cmd
}
