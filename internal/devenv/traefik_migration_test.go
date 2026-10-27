package devenv

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyProxyMigration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		legacy     bool
		fail       string
		wantSuffix []string
	}{
		{name: "fresh"},
		{name: "upgrade", legacy: true, wantSuffix: []string{
			"docker stop legacy-proxy", "compose up -d traefik mediawiki mediawiki-web adminer memcached",
			"docker rm legacy-proxy",
		}},
		{name: "preparation failure preserves nginx", legacy: true, fail: "compose create", wantSuffix: []string{
			"compose create --no-recreate traefik docker-socket-proxy",
		}},
		{name: "startup failure restores nginx", legacy: true, fail: "compose up", wantSuffix: []string{
			"compose up -d traefik mediawiki mediawiki-web adminer memcached", "compose stop traefik", "docker start legacy-proxy",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			runner := func(kind string) gatewayCommand {
				return func(args []string) (string, error) {
					command := kind + " " + strings.Join(args, " ")
					calls = append(calls, command)
					if tc.fail != "" && strings.HasPrefix(command, tc.fail) {
						return "", errors.New("simulated failure")
					}
					if kind == "docker" && args[0] == "ps" {
						if args[1] == "-aq" {
							if !strings.Contains(command, "label=com.docker.compose.service=nginx-proxy") {
								t.Fatal("legacy lookup must be scoped to nginx")
							}
							if tc.legacy {
								return "legacy-proxy\n", nil
							}
							return "", nil
						}
						return "nginx-proxy\nmediawiki\nmediawiki-web\nadminer\nmemcached\n", nil
					}
					if kind == "compose" && args[0] == "--profile" {
						// A stopped service must not be started during migration.
						return "traefik\nmediawiki\nmediawiki-web\nadminer\nmemcached\nphpmyadmin\ncloudflared\n", nil
					}
					return "", nil
				}
			}
			err := (Environment{}).migrateLegacyProxy(runner("docker"), runner("compose"))
			if (err != nil) != (tc.fail != "") {
				t.Fatalf("migration error: %v", err)
			}
			for _, call := range calls {
				if tc.fail != "" && strings.HasPrefix(call, "docker rm ") {
					t.Fatal("failed migration must retain the legacy container")
				}
				if tc.fail == "compose create" && strings.HasPrefix(call, "docker stop ") {
					t.Fatal("failed preparation must leave the legacy proxy running")
				}
			}
			if len(tc.wantSuffix) > 0 {
				if len(calls) < len(tc.wantSuffix) || !reflect.DeepEqual(calls[len(calls)-len(tc.wantSuffix):], tc.wantSuffix) {
					t.Fatalf("unexpected migration sequence: %v", calls)
				}
			} else if len(calls) != 1 {
				t.Fatalf("fresh context should not migrate: %v", calls)
			}
			if !strings.Contains(calls[0], "label=com.docker.compose.project=mwcli-mwdd-") {
				t.Fatal("migration must be scoped to this context")
			}
		})
	}
}
