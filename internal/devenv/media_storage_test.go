package devenv

import "testing"

func TestMediaStorageCredentialsSurviveSynchronization(t *testing.T) {
	original := Context
	t.Cleanup(func() { Context = original })
	m := Environment{BaseDir: t.TempDir()}
	keys := []string{"SWIFT_MEDIA_KEY", "SWIFT_THUMBOR_KEY", "SWIFT_PRIVATE_KEY", "SWIFT_TEMP_URL_KEY", "THUMBOR_PRIVATE_SECRET"}
	credentials := map[string]bool{}
	for _, name := range []string{"first", "second"} {
		Context = name
		m.EnsureReady()
		values := m.Env().List()
		for _, key := range keys {
			value := values[key]
			if len(value) < 26 || credentials[value] {
				t.Fatalf("%s in %s must have an independent random credential", key, name)
			}
			credentials[value] = true
		}
		m.Env().Set("SWIFT_MEDIA_KEY", "explicit-development-key")
		m.EnsureReady()
		if m.Env().Get("SWIFT_MEDIA_KEY") != "explicit-development-key" {
			t.Fatal("synchronization replaced an explicit Swift credential")
		}
		for _, key := range keys[1:] {
			if m.Env().Get(key) != values[key] {
				t.Fatalf("synchronization changed %s", key)
			}
		}
		// Service lifecycle commands inspect these files before invoking Compose.
		for _, service := range []string{"swift", "thumbor"} {
			contents := m.DockerCompose().File(service).Contents()
			if _, ok := contents.Services[service]; !ok {
				t.Fatalf("%s cannot be inspected by the shared service lifecycle", service)
			}
		}
	}
}
