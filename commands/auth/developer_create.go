package auth

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/browser"
)

func NewDeveloperCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Wikimedia developer account and configure it",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := config.State()
			username := c.Effective.Developer.Username
			shouldSetUsername := true

			if username != "" {
				replaceUsername := false
				prompt := &survey.Confirm{
					Message: fmt.Sprintf("A developer username is already configured (%s). Do you want to replace it?", username),
				}
				if err := survey.AskOne(prompt, &replaceUsername); err != nil {
					return err
				}
				if !replaceUsername {
					fmt.Println("Keeping existing username.")
					shouldSetUsername = false
				}
			}

			if shouldSetUsername {
				fmt.Println("This command will guide you through creating a Wikimedia developer account.")
				fmt.Println("We will open the account creation page in your browser.")

				signupURL := "https://idm.wikimedia.org/signup/"
				fmt.Printf("Please create an account at: %s\n", signupURL)

				if err := browser.OpenURL(signupURL); err != nil {
					fmt.Printf("Could not open browser, please open this URL manually: %s\n", signupURL)
				}

				confirmed := false
				prompt := &survey.Confirm{
					Message: "Have you created your account?",
				}
				if err := survey.AskOne(prompt, &confirmed); err != nil {
					return err
				}

				if !confirmed {
					fmt.Println("Aborting.")
					return nil
				}

				for {
					prompt := &survey.Input{
						Message: "Please enter your Wikimedia developer shell username:",
					}
					if err := survey.AskOne(prompt, &username); err != nil {
						return err
					}

					fmt.Printf("Checking if user '%s' exists...\n", username)
					resp, err := http.Get("https://ldap.toolforge.org/user/" + username)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						fmt.Println("User exists!")
						break
					} else if resp.StatusCode == http.StatusNotFound {
						fmt.Println("User not found. Please make sure you have entered the correct username.")
					} else {
						fmt.Printf("Unexpected status code: %d. Could not verify user.\n", resp.StatusCode)
					}
				}

				// Save the username to config
				config.PutKeyValueOnDisk("developer.username", username)
				fmt.Printf("Saved username '%s' to config.\n", username)
			}

			err := setupSSH(username)
			if err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}

func setupSSH(username string) error {
	c := config.State()
	sshKeyPath := c.Effective.Developer.SSHKeyPath
	shouldSetKey := true

	if sshKeyPath != "" {
		replaceKey := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("An SSH key is already configured (%s). Do you want to replace it?", sshKeyPath),
		}
		if err := survey.AskOne(prompt, &replaceKey); err != nil {
			return err
		}
		if !replaceKey {
			fmt.Println("Keeping existing SSH key.")
			shouldSetKey = false
		}
	}

	if shouldSetKey {
		// 1. Ask to setup SSH
		setupSshKey := false
		prompt := &survey.Confirm{
			Message: "Do you want to set up an SSH key now?",
		}
		if err := survey.AskOne(prompt, &setupSshKey); err != nil {
			return err
		}

		if !setupSshKey {
			fmt.Println("You can set up an SSH key later by running this command again.")
			return nil
		}

		// 2. Ask for existing or new key
		keyChoice := ""
		promptSelect := &survey.Select{
			Message: "Do you want to use an existing SSH key or create a new one?",
			Options: []string{"Use an existing key", "Create a new key"},
		}
		if err := survey.AskOne(promptSelect, &keyChoice); err != nil {
			return err
		}

		var publicKeyPath string
		var privateKeyPath string

		if keyChoice == "Use an existing key" {
			// 3. Handle existing key
			for {
				promptInput := &survey.Input{
					Message: "Please enter the path to your private SSH key file:",
					Suggest: func(toComplete string) []string {
						files, _ := filepath.Glob(toComplete + "*")
						return files
					},
				}
				if err := survey.AskOne(promptInput, &privateKeyPath); err != nil {
					return err
				}

				if _, err := os.Stat(privateKeyPath); err == nil {
					publicKeyPath = privateKeyPath + ".pub"
					if _, err := os.Stat(publicKeyPath); err == nil {
						break
					} else {
						fmt.Printf("Public key not found at %s\n", publicKeyPath)
					}
				} else {
					fmt.Println("File not found. Please enter a valid path.")
				}
			}
			config.PutKeyValueOnDisk("developer.ssh_key_path", privateKeyPath)
			fmt.Printf("Saved ssh_key_path '%s' to config.\n", privateKeyPath)

		} else {
			// 4. Handle new key
			generationChoice := ""
			promptSelect := &survey.Select{
				Message: "Do you want to generate a new SSH key yourself, or should I generate one for you?",
				Options: []string{"I will generate it myself", "Generate one for me"},
			}
			if err := survey.AskOne(promptSelect, &generationChoice); err != nil {
				return err
			}

			if generationChoice == "I will generate it myself" {
				fmt.Println("Please generate a new SSH key.")
				fmt.Println("You can find instructions at: https://wikitech.wikimedia.org/wiki/Generate_an_SSH_Key")
				if err := browser.OpenURL("https://wikitech.wikimedia.org/wiki/Generate_an_SSH_Key"); err != nil {
					fmt.Printf("Could not open browser, please open this URL manually: %s\n", "https://wikitech.wikimedia.org/wiki/Generate_an_SSH_Key")
				}

				for {
					promptInput := &survey.Input{
						Message: "Please enter the path to your public SSH key file:",
						Suggest: func(toComplete string) []string {
							files, _ := filepath.Glob(toComplete + "*")
							return files
						},
					}
					if err := survey.AskOne(promptInput, &publicKeyPath); err != nil {
						return err
					}

					if _, err := os.Stat(publicKeyPath); err == nil {
						privateKeyPath = strings.TrimSuffix(publicKeyPath, ".pub")
						if _, err := os.Stat(privateKeyPath); err == nil {
							break
						} else {
							fmt.Printf("Private key not found at %s\n", privateKeyPath)
						}
					} else {
						fmt.Println("File not found. Please enter a valid path.")
					}
				}
				config.PutKeyValueOnDisk("developer.ssh_key_path", privateKeyPath)
				fmt.Printf("Saved ssh_key_path '%s' to config.\n", privateKeyPath)

			} else {
				// CLI generates key
				homeDir, _ := os.UserHomeDir()
				date := time.Now().Format("20060102")
				privateKeyPath = filepath.Join(homeDir, ".ssh", "id_ed25519_wikimedia-"+date)
				publicKeyPath = privateKeyPath + ".pub"

				fmt.Printf("Generating a new ed25519 SSH key at %s\n", privateKeyPath)
				passphrase := ""
				promptPass := &survey.Password{
					Message: "Please enter a passphrase for your new SSH key (recommended):",
				}
				survey.AskOne(promptPass, &passphrase)

				cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", privateKeyPath, "-N", passphrase, "-C", username+"@wikimedia")
				if err := cmd.Run(); err != nil {
					return err
				}
				fmt.Println("SSH key generated successfully.")
				config.PutKeyValueOnDisk("developer.ssh_key_path", privateKeyPath)
				fmt.Printf("Saved ssh_key_path '%s' to config.\n", privateKeyPath)
			}
		}

		// 5. Add public key to Wikimedia
		publicKey, err := os.ReadFile(publicKeyPath)
		if err != nil {
			return err
		}
		fmt.Println("\nPlease add the following public key to your Wikimedia account:")
		fmt.Println("----------------------------------------------------")
		fmt.Println(string(publicKey))
		fmt.Println("----------------------------------------------------")

		keyURL := "https://idm.wikimedia.org/keymanagement/create/"
		fmt.Printf("We will now open the key management page in your browser: %s\n", keyURL)
		if err := browser.OpenURL(keyURL); err != nil {
			fmt.Printf("Could not open browser, please open this URL manually: %s\n", keyURL)
		}

		confirmed := false
		promptConfirm := &survey.Confirm{
			Message: "Have you added the public key to your account?",
		}
		if err := survey.AskOne(promptConfirm, &confirmed); err != nil {
			return err
		}

		if !confirmed {
			fmt.Println("Aborting. You can run this command again to verify the key.")
			return nil
		}

		// 6. Verify the key
		fmt.Println("Verifying the key...")

		// Get local public key data
		publicKey, err = os.ReadFile(publicKeyPath)
		if err != nil {
			return err
		}
		keyParts := strings.Split(string(publicKey), " ")
		if len(keyParts) < 2 {
			return fmt.Errorf("invalid public key format")
		}
		publicKeyData := keyParts[1]

		// Get remote fingerprint
		resp, err := http.Get("https://ldap.toolforge.org/user/" + username)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if strings.Contains(string(body), publicKeyData) {
			fmt.Println("🎉 Congratulations! Your SSH key has been successfully set up and verified. 🎉")
		} else {
			fmt.Println("Could not verify the SSH key. Please make sure you have added the correct key to your account.")
		}
	}

	return nil
}