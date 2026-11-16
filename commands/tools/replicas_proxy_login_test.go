package tools

import (
	"strings"
	"testing"
)

func TestReplicaProxyLoginDefaultsReuseAndOverrides(t *testing.T) {
	replica := replicaCredentials{User: "s12345", Password: "REPLICA_SECRET"}
	for _, test := range []struct {
		name           string
		options        replicaProxyLoginOptions
		user, password string
		generated      bool
	}{
		{name: "defaults", user: "wmuser", generated: true},
		{name: "same account", options: replicaProxyLoginOptions{useReplica: true}, user: "s12345", password: "REPLICA_SECRET"},
		{name: "user only", options: replicaProxyLoginOptions{userSet: true, user: "local_user"}, user: "local_user", generated: true},
		{name: "password only", options: replicaProxyLoginOptions{passwordSet: true, password: "LOCAL_SECRET"}, user: "wmuser", password: "LOCAL_SECRET"},
		{name: "both", options: replicaProxyLoginOptions{userSet: true, user: "local_user", passwordSet: true, password: "LOCAL_SECRET"}, user: "local_user", password: "LOCAL_SECRET"},
		{name: "same with user override", options: replicaProxyLoginOptions{useReplica: true, userSet: true, user: "wmuser"}, user: "wmuser", password: "REPLICA_SECRET"},
		{name: "same with password override", options: replicaProxyLoginOptions{useReplica: true, passwordSet: true, password: "LOCAL_SECRET"}, user: "s12345", password: "LOCAL_SECRET"},
		{name: "same with both overrides", options: replicaProxyLoginOptions{useReplica: true, userSet: true, user: "local_user", passwordSet: true, password: "LOCAL_SECRET"}, user: "local_user", password: "LOCAL_SECRET"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.options.validate(); err != nil {
				t.Fatal(err)
			}
			local, generated := test.options.resolve(replica)
			if local.User != test.user || generated != test.generated {
				t.Fatalf("incorrect user or password source: %q, generated=%v", local.User, generated)
			}
			if generated {
				other, _ := test.options.resolve(replica)
				if len(local.Password) < 26 || local.Password == replica.Password || local.Password == other.Password {
					t.Fatal("password was not generated independently")
				}
			} else if local.Password != test.password {
				t.Fatal("incorrect overridden/reused password")
			}
			description := replicaProxyLoginDescription(local, generated)
			if !strings.Contains(description, local.User) {
				t.Fatal("proxy username was not reported")
			}
			if strings.Contains(description, local.Password) != generated {
				t.Fatal("provided/reused password was exposed, or generated password was hidden")
			}
			if replica.User != "s12345" || replica.Password != "REPLICA_SECRET" {
				t.Fatal("local override changed upstream credentials")
			}
		})
	}
}

func TestReplicaProxyLoginValidationDoesNotEchoSecrets(t *testing.T) {
	for _, options := range []replicaProxyLoginOptions{
		{userSet: true, user: ""},
		{userSet: true, user: "REPLICA_SECRET@host"},
		{passwordSet: true, password: ""},
		{passwordSet: true, password: "REPLICA_SECRET\n"},
		{passwordSet: true, password: "REPLICA_SECRET\x00"},
		{passwordSet: true, password: strings.Repeat("REPLICA_SECRET", 400)},
	} {
		err := options.validate()
		if err == nil || strings.Contains(err.Error(), "REPLICA_SECRET") {
			t.Fatal("invalid local login accepted or echoed")
		}
	}
}
