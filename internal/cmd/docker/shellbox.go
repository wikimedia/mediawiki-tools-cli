package docker

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

//go:embed long/mwdd_shellbox.md
var shellboxLong string

//go:embed long/mwdd_shellbox-media.md
var shellboxLongMedia string

//go:embed long/mwdd_shellbox-php-rpc.md
var shellboxLongPHPRPC string

//go:embed long/mwdd_shellbox-score.md
var shellboxLongScore string

//go:embed long/mwdd_shellbox-syntaxhighlight.md
var shellboxLongSyntaxhighlight string

//go:embed long/mwdd_shellbox-timeline.md
var shellboxLongTimeline string

func NewShellboxCmd() *cobra.Command {
	shellbox := mwdd.NewServicesCmd("shellbox", shellboxLong, []string{})
	shellBoxFlavours := []string{
		"media",
		"php-rpc",
		"score",
		"syntaxhighlight",
		"timeline",
	}
	shellBoxLongDescs := map[string]string{
		"media":           shellboxLongMedia,
		"php-rpc":         shellboxLongPHPRPC,
		"score":           shellboxLongScore,
		"syntaxhighlight": shellboxLongSyntaxhighlight,
		"timeline":        shellboxLongTimeline,
	}
	for _, flavour := range shellBoxFlavours {
		shellboxSubCmd := mwdd.NewServiceCmd(flavour, shellBoxLongDescs[flavour], []string{})
		shellbox.AddCommand(shellboxSubCmd)
		dockerComposeName := "shellbox-" + flavour
		shellboxSubCmd.AddCommand(mwdd.NewServiceCreateCmd(dockerComposeName))
		shellboxSubCmd.AddCommand(mwdd.NewServiceDestroyCmd(dockerComposeName))
		shellboxSubCmd.AddCommand(mwdd.NewServiceSuspendCmd(dockerComposeName))
		shellboxSubCmd.AddCommand(mwdd.NewServiceResumeCmd(dockerComposeName))
	}
	return shellbox
}
