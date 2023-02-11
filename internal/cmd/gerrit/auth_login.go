package gerrit

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/andygrunwald/go-gerrit"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewGerritAuthLoginCmd() *cobra.Command {
	var username string
	var password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Wikimedia Gerrit using HTTP credentials",
		Run: func(cmd *cobra.Command, args []string) {
			if username == "" {
				// Ask what the gerrit username is
				prompt := &survey.Input{
					Message: "Gerrit username:",
				}
				survey.AskOne(prompt, &username)
			} else {
				fmt.Printf("Using username %s, as provided by --username\n", username)
			}

			if password == "" {
				// Ask what the gerrit password is, and provide URL for retrieval
				fmt.Println("Head to https://gerrit.wikimedia.org/r/settings/#HTTPCredentials to retrieve your Gerrit HTTP password")
				prompt := &survey.Password{
					Message: "Gerrit password:",
				}
				survey.AskOne(prompt, &password)
			} else {
				fmt.Print("Using password as provided by --password\n", password)
			}

			// Check the credentials
			instance := "https://gerrit.wikimedia.org/r/"
			client, _ := gerrit.NewClient(instance, nil)
			client.Authentication.SetBasicAuth(username, password)
			_, err := client.Call("GET", "accounts/self/name", nil, nil)
			if err != nil {
				logrus.Errorf("Error making request: %s", err)
				// Ask if they want to ignore the error and save the credentials anyway
				prompt := &survey.Confirm{
					Message: "Error checking credentials, do you want to save the credentials anyway?",
				}
				var ignoreError bool
				survey.AskOne(prompt, &ignoreError)
				if !ignoreError {
					// Exit
					logrus.Error("Credentials not saved, not saved, exiting")
					os.Exit(1)
				}
			}
			logrus.Info("Credentials checked")

			config := &Config{
				Username: username,
				Password: password,
			}
			config.Write()
			logrus.Info("Credentials saved")
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Gerrit username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Gerrit password")
	return cmd
}
