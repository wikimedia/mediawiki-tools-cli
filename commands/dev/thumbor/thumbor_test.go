package thumbor

import (
	osexec "os/exec"
	"path/filepath"
	"testing"
)

func TestSwiftContainerConfiguration(t *testing.T) {
	python, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required to evaluate Thumbor configuration")
	}
	config, err := filepath.Abs("../../../mount/dev/compose/thumbor/mount/thumbor/80-thumbor-swift.conf")
	if err != nil {
		t.Fatal(err)
	}
	cmd := osexec.Command(python, "-c", `
import os, runpy, sys

for name in ('SWIFT_KEY', 'SWIFT_PRIVATE_KEY', 'SWIFT_PRIVATE_SECRET'):
    os.environ[name] = 'test-' + name

cases = [
    ('', '', '', [], []),
    (' one, two , ,three ', ' custom, wikipedia-officewiki-local-public ',
     ' officewiki, teamwiki,officewiki, ', ['one', 'two', 'three'], [
        'custom', 'wikipedia-officewiki-local-public', 'wikipedia-officewiki-local-thumb',
        'wikipedia-officewiki-local-temp', 'wikipedia-officewiki-local-transcoded',
        'wikipedia-teamwiki-local-public', 'wikipedia-teamwiki-local-thumb',
        'wikipedia-teamwiki-local-temp', 'wikipedia-teamwiki-local-transcoded',
     ]),
]
for sharded, private, wikis, expected_sharded, expected_private in cases:
    os.environ['SWIFT_SHARDED_CONTAINERS'] = sharded
    os.environ['SWIFT_PRIVATE_CONTAINERS'] = private
    os.environ['MEDIAWIKI_SWIFT_PRIVATE_WIKIS'] = wikis
    config = runpy.run_path(sys.argv[1])
    assert config['SWIFT_SHARDED_CONTAINERS'] == expected_sharded
    assert config['SWIFT_PRIVATE_CONTAINERS'] == expected_private
    for name in ('SWIFT_KEY', 'SWIFT_PRIVATE_KEY', 'SWIFT_PRIVATE_SECRET'):
        assert config[name] == 'test-' + name
`, config)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("evaluate Thumbor Swift configuration: %v\n%s", err, output)
	}
}
