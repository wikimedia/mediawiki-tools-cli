package quip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewQuipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "quip",
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

			doc.Find(".quote").Each(func(i int, s *goquery.Selection) {
				// link := s.Find("a").First().AttrOr("href", "https://bash.toolforge.org/random")
				s.Find(".nav").Remove()
				fmt.Println(strings.TrimSpace(s.Text()))
				// fmt.Println("https://bash.toolforge.org" + link)
			})
		},
	}
	return cmd
}
