package tools

import (
	"crypto/rand"
	"fmt"
	"strings"
)

type replicaProxyLoginOptions struct {
	useReplica           bool
	user, password       string
	userSet, passwordSet bool
}

func (o replicaProxyLoginOptions) validate() error {
	if o.userSet && !replicaUsernamePattern.MatchString(o.user) {
		return fmt.Errorf("--user must be a username containing letters, digits, underscores, dots or hyphens")
	}
	if o.passwordSet && (o.password == "" || len(o.password) > 4096 || strings.ContainsAny(o.password, "\x00\r\n")) {
		return fmt.Errorf("--password must be nonempty, at most 4096 bytes, and contain no NUL or newline characters")
	}
	return nil
}

func (o replicaProxyLoginOptions) resolve(replica replicaCredentials) (replicaCredentials, bool) {
	local := replicaCredentials{User: "wmuser"}
	generated := !o.useReplica && !o.passwordSet
	if o.useReplica {
		local = replica
	}
	if generated {
		local.Password = rand.Text()
	}
	if o.userSet {
		local.User = o.user
	}
	if o.passwordSet {
		local.Password = o.password
	}
	return local, generated
}

func replicaProxyLoginDescription(local replicaCredentials, generated bool) string {
	if generated {
		return "Proxy user: " + local.User + "  Generated proxy password: " + local.Password
	}
	return "Proxy user: " + local.User + "  Proxy password: configured (not printed)"
}
