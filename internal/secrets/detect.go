package secrets

import "os"

// Availability describes discovery only; it never unlocks a vault or reads a
// credential. An available provider may still need approval when saving.
type Availability struct {
	Available    bool
	Detail       string
	NeedsPairing bool
}

func DetectKeePassXC() Availability {
	if _, err := os.Lstat(KeePassXCPairingPath()); err == nil {
		if _, err := loadBrowserPairing(); err != nil {
			return Availability{Detail: "pairing needs repair; reconnect with config secrets keepassxc connect"}
		}
		return Availability{Available: true, Detail: "paired; the database must be unlocked"}
	}
	if _, err := browserProxy(""); err != nil {
		return Availability{Detail: "not detected; install KeePassXC or configure its proxy"}
	}
	return Availability{Available: true, Detail: "detected; GUI pairing required", NeedsPairing: true}
}
