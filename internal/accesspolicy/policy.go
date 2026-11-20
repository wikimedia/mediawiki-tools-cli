// Package accesspolicy resolves the public-only setting shared by CLI and MCP.
package accesspolicy

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

const (
	Flag                = "public-only"
	ConfigKey           = "access.public_only"
	SupportedAnnotation = "a-wikimedia-cli/public-only"
)

func AddFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(Flag, false, "Restrict supported operations to public data")
}

func Support(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[SupportedAnnotation] = "true"
}

// CLI honors the explicit flag, then the global setting, then an optional legacy setting.
func CLI(cmd *cobra.Command, legacyKey string) (bool, error) {
	if flag := cmd.Flag(Flag); flag != nil && flag.Changed {
		return strconv.ParseBool(flag.Value.String())
	}
	settings := config.State().EffectiveKoanf
	for _, key := range []string{ConfigKey, legacyKey} {
		if key == "" || !settings.Exists(key) {
			continue
		}
		value, err := strconv.ParseBool(settings.String(key))
		if err != nil {
			return false, fmt.Errorf("%s must be true or false", key)
		}
		return value, nil
	}
	return false, nil
}

// MCP is public-only unless the launcher explicitly opts out. A saved CLI
// preference cannot widen an MCP session's access.
func MCP(cmd *cobra.Command) (bool, error) {
	if flag := cmd.Flag(Flag); flag != nil && flag.Changed {
		return strconv.ParseBool(flag.Value.String())
	}
	return true, nil
}
