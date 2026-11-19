package phabricator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
)

const (
	wikimediaPublicSpace = "PHID-SPCE-6l6g5p53yi3mypnlpxjw"
	nonPublicLabel       = "[NON PUBLIC]"
)

type taskAccess struct {
	publicOnly bool
	agent      bool
	endpoint   string
}

func taskAccessFromCommand(cmd *cobra.Command, endpoint string) (taskAccess, error) {
	access := taskAccess{endpoint: strings.TrimRight(endpoint, "/")}
	// The low-level API is deliberately outside these task-command guardrails.
	if cmd.Name() == "api" {
		return access, nil
	}
	agentMode, err := commandAgentMode(cmd)
	if err != nil {
		return access, err
	}
	access.agent = agentMode
	access.publicOnly, err = accesspolicy.CLI(cmd, "phabricator.public_only")
	if err != nil {
		return access, err
	}
	if commandFlagEnabled(cmd, "allow-non-public") {
		if access.agent {
			return access, fmt.Errorf("human approval required: --allow-non-public cannot be used in agent mode; ask a human to rerun the specific operation in their own terminal outside agent mode")
		}
		if commandFlagEnabled(cmd, "public-only") {
			return access, fmt.Errorf("--allow-non-public cannot be combined with --public-only")
		}
		access.publicOnly = false
	}
	access.publicOnly = access.publicOnly || access.agent
	return access, nil
}

func (a taskAccess) permits(task phabTask) bool {
	return !a.publicOnly || a.viewPolicy(task) == "public"
}

func (a taskAccess) viewPolicy(task phabTask) string {
	// A visibility policy is only verified in Wikimedia's known public S1 Space.
	if a.endpoint != defaultPhabricatorURL || task.Fields.SpacePHID != wikimediaPublicSpace {
		return ""
	}
	return task.Fields.Policy["view"]
}

// Allow placeholders only for recognized policies within the supported Space.
func (a taskAccess) canList(task phabTask) bool {
	if !a.publicOnly {
		return true
	}
	switch a.viewPolicy(task) {
	case "public", "users", "admin", "no-one":
		return true
	default:
		return false
	}
}

func (a taskAccess) denied() error {
	if a.agent {
		return fmt.Errorf("human approval required: this operation is non-public or its visibility cannot be verified; do not retry or bypass the restriction; ask a human to rerun the specific operation in their own terminal outside agent mode, using --allow-non-public if needed")
	}
	return fmt.Errorf("public-only access: this operation is non-public or its visibility cannot be verified; to approve it, rerun the specific operation with --allow-non-public outside agent mode")
}

func (c *conduitClient) requirePublicTask(task phabTask) error {
	if !c.access.permits(task) {
		return c.access.denied()
	}
	return nil
}

func (c *conduitClient) taskForList(task phabTask) phabTask {
	if c.access.permits(task) {
		return task
	}
	placeholder := phabTask{Redacted: true, ID: task.ID}
	placeholder.Fields.Name = nonPublicLabel
	placeholder.Fields.Status = task.Fields.Status
	placeholder.Fields.Priority = task.Fields.Priority
	placeholder.Fields.DateCreated = task.Fields.DateCreated
	placeholder.Fields.DateModified = task.Fields.DateModified
	return placeholder
}

func taskReference(task phabTask) string {
	return fmt.Sprintf("T%d", task.ID)
}

func (task phabTask) MarshalJSON() ([]byte, error) {
	if task.Redacted {
		return json.Marshal(map[string]interface{}{
			"redacted": true,
			"id":       task.ID,
			"fields": map[string]interface{}{
				"name":         nonPublicLabel,
				"status":       task.Fields.Status,
				"priority":     task.Fields.Priority,
				"dateCreated":  task.Fields.DateCreated,
				"dateModified": task.Fields.DateModified,
			},
		})
	}
	type plainTask phabTask
	return json.Marshal(plainTask(task))
}
