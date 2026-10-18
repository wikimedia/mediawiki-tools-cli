package docker

import "fmt"

func lifecycleError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("could not %s development containers: %w", action, err)
}
