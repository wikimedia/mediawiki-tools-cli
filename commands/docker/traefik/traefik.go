package traefik

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"golang.org/x/crypto/bcrypt"
)

func NewCmd() *cobra.Command {
	cmd := mwdd.NewServiceCmd("traefik", mwdd.ServiceTexts{
		Long: "An optional Traefik gateway for testing path-based access to development services. " +
			"It listens on localhost port 8081 by default and does not replace mwdd's nginx proxy.",
	}, []string{})
	cmd.Long = "An optional Traefik gateway for testing path-based access to development services. " +
		"Start it with 'mw docker traefik create'. It listens on localhost port 8081 by default, " +
		"leaving mwdd's nginx proxy and port 8080 unchanged. It discovers services through their " +
		"Traefik Compose labels and a read-only Docker API proxy. Gateway authentication is enabled " +
		"by default; the first create generates a password. Set TRAEFIK_AUTH_MIDDLEWARE=forwarded-https@file " +
		"in the context environment to disable authentication. Configure TRAEFIK_BIND_ADDR and " +
		"TRAEFIK_PORT to change the listener."

	create, _, err := cmd.Find([]string{"create"})
	if err == nil {
		create.PreRunE = func(cmd *cobra.Command, args []string) error {
			return prepareGatewayAuth()
		}
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "credentials",
		Short: "Show the generated gateway login",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(mwdd.DefaultForUser().Directory(), "compose", "traefik", "auth", "credentials")
			credentials, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("gateway credentials are not available yet; create the Traefik service first: %w", err)
			}
			fmt.Print(strings.TrimSpace(string(credentials)) + "\n")
			return nil
		},
	})
	return cmd
}

func prepareGatewayAuth() error {
	instance := mwdd.DefaultForUser()
	instance.EnsureReady()

	authDir := filepath.Join(instance.Directory(), "compose", "traefik", "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return err
	}
	usersPath := filepath.Join(authDir, "users")
	credentialsPath := filepath.Join(authDir, "credentials")
	if _, err := os.Stat(usersPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	passwordBytes := make([]byte, 24)
	if _, err := rand.Read(passwordBytes); err != nil {
		return err
	}
	password := base64.RawURLEncoding.EncodeToString(passwordBytes)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := os.WriteFile(usersPath, []byte("mwcli:"+string(hash)+"\n"), 0o600); err != nil {
		return err
	}
	credentials := fmt.Sprintf("Username: mwcli\nPassword: %s\n", password)
	if err := os.WriteFile(credentialsPath, []byte(credentials), 0o600); err != nil {
		_ = os.Remove(usersPath)
		return err
	}
	fmt.Printf("Traefik gateway credentials generated and saved to %s\n%s", credentialsPath, credentials)
	return nil
}
