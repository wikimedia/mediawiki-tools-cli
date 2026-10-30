package secrets

// Availability describes discovery only; it never unlocks a vault.
type Availability struct {
	Available    bool
	Detail       string
	NeedsPairing bool
}

func DetectKeePassXC() Availability {
	status := browserClient().Detect()
	return Availability{Available: status.Available, Detail: status.Detail, NeedsPairing: status.NeedsPairing}
}
