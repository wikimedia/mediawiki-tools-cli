package swift

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
)

func TestBootstrapSecuresExistingPrivateContainers(t *testing.T) {
	php, err := osexec.LookPath("php")
	if err != nil {
		t.Skip("PHP is required to exercise the Swift maintenance helper")
	}
	install := t.TempDir()
	maintenance := filepath.Join(install, "maintenance")
	if err := os.Mkdir(maintenance, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(maintenance, "Maintenance.php"), []byte("<?php\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MW_INSTALL_PATH", install)
	helper, err := filepath.Abs("../../../mount/dev/compose/swift/mount/prepareSwift.php")
	if err != nil {
		t.Fatal(err)
	}
	cmd := osexec.Command(php, "-r", `
namespace MediaWiki {
    class MainConfigNames { const LocalFileRepo = 'LocalFileRepo'; }
}
namespace Wikimedia\FileBackend {
    class SwiftFileBackend {
        public array $calls = [];
        public array $containers = ['public' => true];
        public function prepare($params) {
            $zone = substr($params['dir'], strlen('mwstore://test/local-'));
            $this->calls[] = ['prepare', $zone, $params];
            // Preparing an existing container does not change its ACL.
            $this->containers[$zone] ??= empty($params['noAccess']);
            return new \GoodStatus;
        }
        public function secure($params) {
            $zone = substr($params['dir'], strlen('mwstore://test/local-'));
            if (!array_key_exists($zone, $this->containers)) {
                throw new \RuntimeException('secure called before prepare');
            }
            $this->calls[] = ['secure', $zone, $params];
            $this->containers[$zone] = false;
            return new \GoodStatus;
        }
    }
}
namespace MediaWiki\Maintenance {
    class Maintenance {
        public static $repo;
        public static bool $private = false;
        public function __construct() {}
        public function addDescription($text) {}
        public function getServiceContainer() { return $this; }
        public function getRepoGroup() { return $this; }
        public function getLocalRepo() { return self::$repo; }
        public function getConfig() { return $this; }
        public function get($name) { return ['isPrivate' => self::$private]; }
        public function output($text) {}
        public function fatalError($text) { throw new \RuntimeException($text); }
    }
}
namespace {
    class GoodStatus { public function isOK() { return true; } }
    class Repo {
        public $backend;
        public function __construct() { $this->backend = new \Wikimedia\FileBackend\SwiftFileBackend; }
        public function getBackend() { return $this->backend; }
        public function getZonePath($zone) { return 'mwstore://test/local-' . $zone; }
    }
    $class = require $argv[1];
    foreach ([false, true] as $private) {
        \MediaWiki\Maintenance\Maintenance::$private = $private;
        $repo = new Repo;
        \MediaWiki\Maintenance\Maintenance::$repo = $repo;
        $helper = new $class;
        $helper->execute();
        $zones = $private ? ['public', 'thumb', 'temp', 'transcoded', 'deleted'] : ['thumb'];
        foreach ($zones as $zone) {
            $prepare = array_shift($repo->backend->calls);
            if ($prepare[0] !== 'prepare' || $prepare[1] !== $zone) {
                throw new \RuntimeException('zone not prepared: ' . $zone);
            }
            if ($private) {
                $secure = array_shift($repo->backend->calls);
                if ($secure[0] !== 'secure' || $secure[1] !== $zone || empty($secure[2]['noAccess']) ||
                    empty($secure[2]['noListing']) || $repo->backend->containers[$zone] !== false) {
                    throw new \RuntimeException('private zone not secured: ' . $zone);
                }
            }
        }
        if ($repo->backend->calls) { throw new \RuntimeException('unexpected operations'); }
        if (!$private && $repo->backend->containers['public'] !== true) {
            throw new \RuntimeException('public storage changed');
        }
    }
}
`, helper)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("exercise Swift bootstrap: %v\n%s", err, output)
	}
}
