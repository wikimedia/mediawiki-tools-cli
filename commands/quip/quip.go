package quip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

func NewQuipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quip",
		Short: "Outputs a quip from bash.toolforge.org",
		Example: cobrautil.NormalizeExample(`quip
quip --link`),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := http.NewRequest("GET", "https://bash.toolforge.org/random", nil)
			if err != nil {
				return fmt.Errorf("create quip request: %w", err)
			}

			ctx := context.Background()
			c := http.Client{}
			req = req.WithContext(ctx)

			req.Header.Set("User-Agent", cli.HTTPUserAgent)
			res, err := c.Do(req)
			if err != nil {
				return fmt.Errorf("request quip: %w", err)
			}

			defer res.Body.Close()

			if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
				b, err := io.ReadAll(res.Body)
				if err != nil {
					return fmt.Errorf("read quip response: %w", err)
				}
				return fmt.Errorf("quip service returned HTTP %d: %s", res.StatusCode, string(b))
			}

			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				return fmt.Errorf("parse quip response: %w", err)
			}

			firstQuote := doc.Find(".quote").First()
			link := "https://bash.toolforge.org" + firstQuote.Find("a").First().AttrOr("href", "/random")
			firstQuote.Find(".nav").Remove()
			text := firstQuote.Text()

			hasCowsay := lookpath.HasExecutable("cowsay")
			hasLolcat := lookpath.HasExecutable("lolcat")
			noFun, _ := cmd.Flags().GetBool("no-fun")

			if (!hasCowsay && !hasLolcat) || noFun {
				fmt.Println(text)
				return nil
			}

			// Lets have some fun
			tmpFile := files.StringToTempFile(text)
			defer os.Remove(tmpFile)

			cmds := "cat " + tmpFile
			if hasCowsay {
				cmds += " | cowsay -n"
			}
			if hasLolcat {
				cmds += " | lolcat"
			}

			execCmd := exec.Command("bash", "-c", cmds) // #nosec G204
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			if err := execCmd.Run(); err != nil {
				return fmt.Errorf("display quip: %w", err)
			}

			// Finally output the link if requested
			printLink, _ := cmd.Flags().GetBool("link")
			if printLink {
				fmt.Println(link)
			}
			return nil
		},
	}
	cmd.Flags().BoolP("no-fun", "n", false, "disable fun")
	cmd.Flags().BoolP("link", "", false, "output a link to the quip")
	return cmd
}
