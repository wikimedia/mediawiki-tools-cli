package codesearch

import (
	"context"
	"fmt"
	"net/http"
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
	IgnoreCase bool
}

func (c *Client) Search(ctx context.Context, flavour string, query string, options *SearchOptions) (*SearchResponse, error) {
	ignoreCase := ""
	if options != nil && options.IgnoreCase {
		ignoreCase = "fosho"
	} else {
		ignoreCase = "nope"
	}

	url := fmt.Sprintf("%s?q=%s&repos=*&i=%s&stats=fosho", BaseURLForFlavour(flavour), query, ignoreCase)
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
