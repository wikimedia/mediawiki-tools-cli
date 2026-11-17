package commandmeta

import (
	"fmt"

	"github.com/spf13/cobra"
)

type AgentPolicy string

const (
	PolicyPermissive AgentPolicy = "permissive"
	PolicyBalanced   AgentPolicy = "balanced"
	PolicyCautious   AgentPolicy = "cautious"
)

func ParseAgentPolicy(value string) (AgentPolicy, error) {
	switch policy := AgentPolicy(value); policy {
	case PolicyPermissive, PolicyBalanced, PolicyCautious:
		return policy, nil
	default:
		return "", fmt.Errorf("agent.policy must be permissive, balanced or cautious, got %q", value)
	}
}

// CheckAgentPolicy accepts cooperative approval, not proof of human consent.
// preview must only be true for a caller-verified, side-effect-free preview path.
func CheckAgentPolicy(cmd *cobra.Command, policy AgentPolicy, agent, preview, approved bool) error {
	if !agent {
		return nil
	}
	if _, err := ParseAgentPolicy(string(policy)); err != nil {
		return err
	}
	hints, err := Read(cmd)
	if err != nil {
		return fmt.Errorf("agent policy: %w", err)
	}
	if preview {
		return nil
	}
	highRisk := hints.HighRisk || hints.SecretInput
	for _, category := range hints.DataAccess {
		highRisk = highRisk || category == DataSecrets || category == DataUnknown
	}
	requireApproval := highRisk || policy == PolicyCautious || (policy == PolicyBalanced && !hints.ReadOnlyHint)
	if !requireApproval || approved {
		return nil
	}
	previewAdvice := ""
	if cmd.Flags().Lookup("dry-run") != nil {
		previewAdvice = " Run the same operation with --dry-run and show the preview to the user."
	}
	return fmt.Errorf("human approval required: %s requires user confirmation under agent.policy=%s.%s Explain the exact operation, target, supplied input and relevant risks, then ask the user to confirm; only after approval rerun the same operation with --yes. --yes asserts approval but does not verify it or override task visibility checks", cmd.CommandPath(), policy, previewAdvice)
}
