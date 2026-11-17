package agent

import (
	"errors"
	"fmt"
	"os"
	"strings"

	detectagent "github.com/vercel/detect-agent"
)

// Detect returns nil when no agent is detected.
func Detect() (*detectagent.AgentDetails, error) {
	detected, err := detectagent.Detect()
	if err != nil && !errors.Is(err, detectagent.ErrAgentNotFound) {
		return nil, fmt.Errorf("detect agent: %w", err)
	}
	if detected != nil {
		return detected, nil
	}
	// Keep recognizing the Copilot SDK marker not yet covered upstream.
	if value := strings.TrimSpace(os.Getenv("COPILOT_AGENT_SESSION_ID")); value != "" && value != "0" && !strings.EqualFold(value, "false") {
		return &detectagent.AgentDetails{Name: "github-copilot"}, nil
	}
	return nil, nil
}
