package commandmeta

import (
	"fmt"

	"github.com/spf13/cobra"
)

// CheckAgentPolicy applies the initial agent policy to classified commands.
// preview must only be true for a caller-verified, side-effect-free preview path.
func CheckAgentPolicy(cmd *cobra.Command, agent, preview bool) error {
	if !agent {
		return nil
	}
	hints, err := Read(cmd)
	if err != nil {
		return fmt.Errorf("agent policy: %w", err)
	}
	blocked := hints.SecretInput
	for _, category := range hints.DataAccess {
		blocked = blocked || category == DataSecrets || category == DataUnknown
	}
	if !blocked && (hints.ReadOnlyHint || preview) {
		return nil
	}
	return fmt.Errorf("human approval required: %s is not permitted in agent mode; --yes does not override agent policy; do not retry or bypass this restriction; ask a human to rerun the specific operation in their own terminal outside agent mode", cmd.CommandPath())
}
