package codesearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

type SearchResponse struct {
	Results map[string]ResultObject `json:"Results"`
	Stats   struct {
		FilesOpened int `json:"FilesOpened"`
		Duration    int `json:"Duration"`
	} `json:"Stats"`
}

type ResultObject struct {
	Matches []struct {
		Filename string `json:"Filename"`
		Matches  []struct {
			Line       string   `json:"Line"`
			LineNumber int      `json:"LineNumber"`
			Before     []string `json:"Before"`
			After      []string `json:"After"`
		} `json:"Matches"`
	} `json:"Matches"`
	FilesWithMatch int    `json:"FilesWithMatch"`
	Revision       string `json:"Revision"`
}

type SearchOptions struct {
	IgnoreCase   bool
	Files        string
	ExcludeFiles string
	Repos        []string
}

func (c *Client) Search(ctx context.Context, flavour string, query string, options *SearchOptions) (*SearchResponse, error) {
	params := url.Values{}

	params.Add("q", query)
	params.Add("stats", "fosho")

	if options != nil && options.IgnoreCase {
		params.Add("i", "fosho")
	} else {
		params.Add("i", "nope")
	}

	if options != nil && options.Files != "" {
		params.Add("files", options.Files)
	}

	if options != nil && options.ExcludeFiles != "" {
		params.Add("excludeFiles", options.ExcludeFiles)
	}

	if options != nil && len(options.Repos) > 0 {
		params.Add("repos", strings.Join(options.Repos, ","))
	} else {
		params.Add("repos", "*")
	}

	url := fmt.Sprintf("%s?%s", BaseURLForFlavour(flavour), params.Encode())
	logrus.Debugf("URL: %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	res := SearchResponse{}
	if err := c.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}
