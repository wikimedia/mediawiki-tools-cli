package shellbox

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed shellbox.long.md
var shellboxLong string

//go:embed shellbox-media.long.md
var shellboxLongMedia string

//go:embed shellbox-php-rpc.long.md
var shellboxLongPHPRPC string

//go:embed shellbox-score.long.md
var shellboxLongScore string

//go:embed shellbox-syntaxhighlight.long.md
var shellboxLongSyntaxhighlight string

//go:embed shellbox-timeline.long.md
var shellboxLongTimeline string

func NewCmd() *cobra.Command {
	shellbox := mwdd.NewServicesCmd("shellbox", mwdd.ServiceTexts{Long: shellboxLong}, []string{})
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
		serviceName := "shellbox-" + flavour
		shellboxSubCmd := mwdd.NewServiceCmdDifferingNames(flavour, serviceName, mwdd.ServiceTexts{Long: shellBoxLongDescs[flavour]}, []string{})
		shellbox.AddCommand(shellboxSubCmd)
	}
	return shellbox
}
