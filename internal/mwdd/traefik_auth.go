package mwdd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// RemoveLegacyNginxProxy removes a proxy container left behind by an older
// Compose manifest so it cannot hold Traefik's listener port during upgrade.
func RemoveLegacyNginxProxy() error {
	project := DefaultForUser().DockerCompose().Name
	args := []string{"ps", "-aq", "--filter", "label=com.docker.compose.project=" + project, "--filter", "label=com.docker.compose.service=nginx-proxy"}
	output, err := exec.Command("docker", args...).Output()
	if err != nil {
		return fmt.Errorf("find the previous nginx proxy container: %w", err)
	}
	containers := strings.Fields(string(output))
	if len(containers) == 0 {
		return nil
	}
	removeArgs := append([]string{"rm", "--force"}, containers...)
	remove := exec.Command("docker", removeArgs...)
	remove.Stdout = os.Stdout
	remove.Stderr = os.Stderr
	return remove.Run()
}

// EnsureTraefikAuth creates private credentials for the default Traefik proxy.
// Existing credentials are preserved across restarts and file syncs.
func EnsureTraefikAuth() error {
	instance := DefaultForUser()
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
	fmt.Printf("Traefik proxy credentials generated and saved to %s\n%s", credentialsPath, strings.TrimSpace(credentials)+"\n")
	return nil
}
