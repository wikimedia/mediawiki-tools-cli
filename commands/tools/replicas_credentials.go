package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
)

type replicaCredentials struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// A fixed, bounded script returns only the required fields. Parser exceptions
// can contain secret file contents, so neither Python nor Go prints them.
const replicaCredentialsScript = `import configparser, json, os, re, sys
try:
    config = configparser.ConfigParser(interpolation=None)
    with open(os.path.expanduser('~/replica.my.cnf'), encoding='utf-8') as source:
        data = source.read(16385)
    if len(data) > 16384:
        raise ValueError()
    config.read_string(data)
    def field(name):
        value = config.get('client', name).strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in ('"', "'"):
            value = value[1:-1]
        return value
    user, password = field('user'), field('password')
    if not re.fullmatch(r'[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}', user):
        raise ValueError()
    if not password or len(password) > 4096 or any(c in password for c in '\x00\r\n'):
        raise ValueError()
except Exception:
    print('Could not read replica credentials from ~/replica.my.cnf.', file=sys.stderr)
    sys.exit(1)
print(json.dumps({'user': user, 'password': password}))
`

func readReplicaCredentials(ctx context.Context, connection toolforge.Connection, input io.Reader, stderr io.Writer) (replicaCredentials, error) {
	value, err := readReplicaFields(ctx, connection, input, stderr, replicaCredentialsScript)
	if err != nil {
		return replicaCredentials{}, fmt.Errorf("could not retrieve replica credentials over SSH")
	}
	var credentials replicaCredentials
	err = json.Unmarshal(value, &credentials)
	clear(value)
	if err != nil || !replicaUsernamePattern.MatchString(credentials.User) || credentials.Password == "" || len(credentials.Password) > 4096 || strings.ContainsAny(credentials.Password, "\x00\r\n") {
		return replicaCredentials{}, fmt.Errorf("Toolforge returned invalid replica credentials")
	}
	return credentials, nil
}
