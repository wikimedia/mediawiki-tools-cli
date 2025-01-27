package codesearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	DefaultFlavour = "search"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func BaseURLForFlavour(flavour string, isApi bool) string {
	if isApi {
		return "https://codesearch.wmcloud.org/" + flavour + "/api/v1/search"
	}
	return "https://codesearch.wmcloud.org/" + flavour + "/"
}

func NewClient(flavour string) *Client {
	return &Client{
		BaseURL: BaseURLForFlavour(flavour, true),
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func CraftSearchURL(flavour string, isApi bool, query string, options *SearchOptions) string {
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

	return fmt.Sprintf("%s?%s", BaseURLForFlavour(flavour, isApi), params.Encode())
}

func (c *Client) sendRequest(req *http.Request, v interface{}) error {
	req.Header.Set("User-Agent", "mwcli codesearch")
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			logrus.Fatalln(err)
		}

		return fmt.Errorf("unknown error, status code: %d, raw result: %s", res.StatusCode, string(b))
	}

	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		return err
	}

	return nil
}
