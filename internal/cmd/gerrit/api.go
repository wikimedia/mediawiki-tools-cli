package gerrit

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/andygrunwald/go-gerrit"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//go:embed api.example
var apiExample string

func NewGerritAPICmd() *cobra.Command {
	config := LoadConfig()

	var (
		method   string
		user     string
		password string
	)

	cmd := &cobra.Command{
		Use:     "api",
		Short:   "Gerrit's API",
		Example: apiExample,
		Long:    `https://gerrit.wikimedia.org/r/Documentation/rest-api.html`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(1)
			}

			if user == "" {
				logrus.Trace("Using username from config")
				user = config.Username
			}
			if password == "" {
				logrus.Trace("Using password from config")
				password = config.Password
			}

			instance := "https://gerrit.wikimedia.org/r/"
			client, _ := gerrit.NewClient(instance, nil)
			if user != "" && password != "" {
				client.Authentication.SetBasicAuth(user, password)
				logrus.Trace("Using username and password")
			}

			resp, err := client.Call(method, args[0], nil, nil)
			if err != nil {
				logrus.Fatalf("Error making request: %s", err)
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
	cmd.Flags().StringVarP(&method, "method", "X", "GET", "The HTTP method for the request")
	cmd.Flags().StringVarP(&user, "auth-user", "", "", "Gerrit HTTP user")
	cmd.Flags().StringVarP(&password, "auth-password", "", "", "Gerrit HTTP password")
	return cmd
}
