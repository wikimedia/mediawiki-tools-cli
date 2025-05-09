package quip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

func NewQuipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quip",
		Short: "Outputs a quip from bash.toolforge.org",
		Example: `quip
quip --link`,
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			req, err := http.NewRequest("GET", "https://bash.toolforge.org/random", nil)
			if err != nil {
				panic(err)
			}

			ctx := context.Background()
			c := http.Client{}
			req = req.WithContext(ctx)

			req.Header.Set("User-Agent", "mwcli quip")
			res, err := c.Do(req)
			if err != nil {
				panic(err)
			}

			defer res.Body.Close()

			if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
				b, err := io.ReadAll(res.Body)
				if err != nil {
					logrus.Fatalln(err)
				}

				panic(fmt.Sprintf("unknown error, status code: %d, raw result: %s", res.StatusCode, string(b)))
			}

			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				panic(err)
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
				return
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
				panic(err)
			}

			// Finally output the link if requested
			printLink, _ := cmd.Flags().GetBool("link")
			if printLink {
				fmt.Println(link)
			}
		},
	}
	cmd.Flags().BoolP("no-fun", "n", false, "disable fun")
	cmd.Flags().BoolP("link", "", false, "output a link to the quip")
	return cmd
}
