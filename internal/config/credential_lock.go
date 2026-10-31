package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gofrs/flock"
	"github.com/knadh/koanf"
	koanfjson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
)

// Serialize external credential mutations and refresh the registry after taking
// the lock, so cleanup cannot race a transfer or save in another CLI process.
func lockCredentials(ctx context.Context) (func(), error) {
	State()
	if err := secureConfigPath(); err != nil {
		return nil, err
	}
	path := Path() + ".credentials.lock"
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("credential lock must be a regular file")
	}
	lock := flock.New(path)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !ok {
		return nil, fmt.Errorf("could not acquire credential mutation lock")
	}
	disk := koanf.New(".")
	if err := disk.Load(file.Provider(Path()), koanfjson.Parser()); err != nil {
		_ = lock.Unlock()
		return nil, fmt.Errorf("could not reload credential registry")
	}
	ReApplyKoanfConf(disk)
	return func() { _ = lock.Unlock() }, nil
}
