package config

import (
	"strings"
	"testing"
)

func TestNewNativeCredentialsUseAWikimediaNamespaceWithoutFallback(t *testing.T) {
	store := isolatedConfig(t)
	credential := seedCredential(t)
	if err := credential.Save("fixture-secret", false); err != nil {
		t.Fatal(err)
	}
	var ref credentialReference
	key := credentialReferenceKey(credential.Key)
	if err := State().OnDiskKoanf.Unmarshal(key, &ref); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ref.Service, "a-wikimedia-cli:") {
		t.Fatalf("unexpected namespace: %s", ref.Service)
	}
	// Development data in the abandoned namespace must not be silently used.
	ref.Service = strings.Replace(ref.Service, "a-wikimedia-cli:", "mwcli:", 1)
	disk := State().OnDiskKoanf.Copy()
	if err := disk.Set(key, ref); err != nil {
		t.Fatal(err)
	}
	if err := PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	reads := store.reads
	if _, err := credential.Read(); err == nil {
		t.Fatal("old namespace was accepted")
	}
	if store.reads != reads {
		t.Fatal("old namespace was contacted")
	}
}
