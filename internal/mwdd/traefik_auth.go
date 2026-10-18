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

// gatewayCommand allows migration ordering and rollback to be tested without
// shell scripts or a Docker daemon (including on Windows).
type gatewayCommand func([]string) (string, error)

// migrateLegacyProxy recreates running services with their new routing labels
// before retiring nginx. Stopped applications are not selected for migration.
func (m MWDD) migrateLegacyProxy(docker, compose gatewayCommand) error {
	projectFilter := "label=com.docker.compose.project=" + strings.ToLower(m.DockerCompose().Name)
	output, err := docker([]string{"ps", "-aq", "--filter", projectFilter, "--filter", "label=com.docker.compose.service=nginx-proxy"})
	if err != nil {
		return fmt.Errorf("find the previous nginx proxy container: %w", err)
	}
	containers := strings.Fields(output)
	if len(containers) == 0 {
		return nil
	}
	fmt.Println("Migrating the development proxy from nginx to Traefik...")
	output, err = docker([]string{"ps", "--filter", projectFilter, "--format", `{{.Label "com.docker.compose.service"}}`})
	if err != nil {
		return fmt.Errorf("find running services for proxy migration: %w", err)
	}
	running := strings.Fields(output)
	available, err := compose([]string{"--profile", "*", "config", "--services"})
	if err != nil {
		return fmt.Errorf("validate proxy migration configuration: %w", err)
	}
	known := make(map[string]bool)
	for _, name := range strings.Fields(available) {
		known[name] = true
	}
	services := []string{"traefik"}
	nginxRunning := false
	for _, name := range running {
		if name == "nginx-proxy" {
			nginxRunning = true
			continue
		}
		if !known[name] {
			return fmt.Errorf("cannot migrate running service %s: missing from Compose configuration", name)
		}
		if name != "traefik" {
			services = append(services, name)
		}
	}
	// Prepare proxy containers and missing images without interrupting the working
	// listener. Creating a container does not bind its published port.
	if _, err := compose([]string{"create", "--no-recreate", "traefik", "gateway-auth", "docker-socket-proxy"}); err != nil {
		return fmt.Errorf("prepare proxy migration containers: %w", err)
	}
	if _, err := docker(append([]string{"stop"}, containers...)); err != nil {
		return fmt.Errorf("stop the previous nginx proxy: %w", err)
	}
	if _, err := compose(append([]string{"up", "-d"}, services...)); err != nil {
		// Keep the previous container until the replacement starts successfully.
		_, stopErr := compose([]string{"stop", "traefik"})
		if stopErr != nil {
			return fmt.Errorf("migrate web proxy: %w; stop replacement before recovery: %v", err, stopErr)
		}
		if nginxRunning {
			if _, restartErr := docker(append([]string{"start"}, containers...)); restartErr != nil {
				return fmt.Errorf("migrate web proxy: %w; restore nginx: %v", err, restartErr)
			}
		}
		return fmt.Errorf("migrate web proxy (previous nginx container retained): %w", err)
	}
	if _, err := docker(append([]string{"rm"}, containers...)); err != nil {
		return fmt.Errorf("retire the previous nginx proxy: %w", err)
	}
	return nil
}

// EnsureTraefikAuth creates private credentials for the default Traefik proxy.
// Existing credentials are preserved across restarts and file syncs.
func EnsureTraefikAuth() error {
	instance := DefaultForUser()
	instance.EnsureReady()
	return instance.ensureTraefikAuth()
}

// PrepareGateway handles both fresh starts and upgrades of existing contexts.
func (m MWDD) PrepareGateway() error {
	m.EnsureReady()
	if err := m.ensureTraefikAuth(); err != nil {
		return err
	}
	return m.migrateLegacyProxy(func(args []string) (string, error) {
		output, err := exec.Command("docker", args...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%w: %s", err, output)
		}
		return string(output), nil
	}, func(args []string) (string, error) {
		stdout, stderr, err := m.DockerCompose().Command(args).RunAndCollect()
		if err != nil {
			return "", fmt.Errorf("%w: %s%s", err, stdout.String(), stderr.String())
		}
		return stdout.String(), nil
	})
}

func (m MWDD) ensureTraefikAuth() error {
	authDir := filepath.Join(m.Directory(), "compose", "traefik", "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return err
	}
	if err := ensureTraefikSessionKey(authDir); err != nil {
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

// Keep the signing key private and stable across gateway restarts.
func ensureTraefikSessionKey(authDir string) error {
	path := filepath.Join(authDir, "session-key")
	if key, err := os.ReadFile(path); err == nil {
		if len(key) != 32 {
			return fmt.Errorf("invalid gateway session key: %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(key); err != nil {
		file.Close()
		os.Remove(path)
		return err
	}
	return file.Close()
}
