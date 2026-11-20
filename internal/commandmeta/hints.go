// Package commandmeta describes command capabilities without enforcing access policy.
package commandmeta

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	ReadOnlyAnnotation    = "mwcli-hint-read-only"
	DestructiveAnnotation = "mwcli-hint-destructive"
	IdempotentAnnotation  = "mwcli-hint-idempotent"
	OpenWorldAnnotation   = "mwcli-hint-open-world"
	DataAccessAnnotation  = "mwcli-hint-data-access"
	SecretInputAnnotation = "mwcli-hint-secret-input"
	HighRiskAnnotation    = "mwcli-hint-high-risk"
)

type DataAccess string

const (
	DataNone            DataAccess = "none"
	DataPublic          DataAccess = "public"
	DataPrivateContent  DataAccess = "potentially-private-content"
	DataPrivateMetadata DataAccess = "potentially-private-metadata"
	DataAccountMetadata DataAccess = "account-metadata"
	DataSuppliedContent DataAccess = "supplied-content"
	DataSecrets         DataAccess = "secrets"
	DataUnknown         DataAccess = "unknown"
)

// ToolAnnotations uses MCP field names; these hints are not authorization.
type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

type Hints struct {
	ToolAnnotations
	DataAccess  []DataAccess `json:"dataAccess"`
	SecretInput bool         `json:"secretInput"`
	HighRisk    bool         `json:"highRisk"`
}

// IsHighRisk reports whether the operation handles sensitive or unknown data or is explicitly marked high risk.
func (h Hints) IsHighRisk() bool {
	if h.HighRisk || h.SecretInput {
		return true
	}
	for _, category := range h.DataAccess {
		if category == DataSecrets || category == DataUnknown {
			return true
		}
	}
	return false
}

var ErrMissingHints = errors.New("command capability hints are missing")

// Set attaches command-local hints, preserving unrelated annotations.
func Set(cmd *cobra.Command, hints Hints) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = Annotations(hints)
		return cmd
	}
	for key, value := range Annotations(hints) {
		cmd.Annotations[key] = value
	}
	return cmd
}

// Annotations encodes typed hints for a Cobra command definition.
func Annotations(hints Hints) map[string]string {
	data := make([]string, len(hints.DataAccess))
	for i, access := range hints.DataAccess {
		data[i] = string(access)
	}
	return map[string]string{
		ReadOnlyAnnotation:    strconv.FormatBool(hints.ReadOnlyHint),
		DestructiveAnnotation: strconv.FormatBool(hints.DestructiveHint),
		IdempotentAnnotation:  strconv.FormatBool(hints.IdempotentHint),
		OpenWorldAnnotation:   strconv.FormatBool(hints.OpenWorldHint),
		DataAccessAnnotation:  strings.Join(data, ","),
		SecretInputAnnotation: strconv.FormatBool(hints.SecretInput),
		HighRiskAnnotation:    strconv.FormatBool(hints.HighRisk),
	}
}

// Read requires a complete classification on this command, never its parent.
func Read(cmd *cobra.Command) (Hints, error) {
	var hints Hints
	found := false
	for _, key := range []string{ReadOnlyAnnotation, DestructiveAnnotation, IdempotentAnnotation, OpenWorldAnnotation, DataAccessAnnotation, SecretInputAnnotation, HighRiskAnnotation} {
		if _, ok := cmd.Annotations[key]; ok {
			found = true
		}
	}
	if !found {
		return hints, ErrMissingHints
	}
	for _, field := range []struct {
		key   string
		value *bool
	}{
		{ReadOnlyAnnotation, &hints.ReadOnlyHint},
		{DestructiveAnnotation, &hints.DestructiveHint},
		{IdempotentAnnotation, &hints.IdempotentHint},
		{OpenWorldAnnotation, &hints.OpenWorldHint},
		{SecretInputAnnotation, &hints.SecretInput},
		{HighRiskAnnotation, &hints.HighRisk},
	} {
		value, err := strconv.ParseBool(cmd.Annotations[field.key])
		if err != nil {
			return hints, fmt.Errorf("%s must be a boolean: %w", field.key, err)
		}
		*field.value = value
	}
	for _, value := range strings.Split(cmd.Annotations[DataAccessAnnotation], ",") {
		hints.DataAccess = append(hints.DataAccess, DataAccess(value))
	}
	return hints, hints.Validate()
}

// Validate checks a classification regardless of how it was constructed.
func (h Hints) Validate() error {
	if len(h.DataAccess) == 0 {
		return fmt.Errorf("%s must contain a data category", DataAccessAnnotation)
	}
	seen := make(map[DataAccess]bool)
	for _, access := range h.DataAccess {
		switch access {
		case DataNone, DataPublic, DataPrivateContent, DataPrivateMetadata, DataAccountMetadata, DataSuppliedContent, DataSecrets, DataUnknown:
		default:
			return fmt.Errorf("%s contains invalid data category %q", DataAccessAnnotation, access)
		}
		if seen[access] {
			return fmt.Errorf("%s contains duplicate category %q", DataAccessAnnotation, access)
		}
		seen[access] = true
	}
	if len(h.DataAccess) > 1 && (seen[DataNone] || seen[DataUnknown]) {
		return fmt.Errorf("%s categories none and unknown must be used alone", DataAccessAnnotation)
	}
	if h.ReadOnlyHint && h.DestructiveHint {
		return fmt.Errorf("read-only commands cannot also be destructive")
	}
	return nil
}
