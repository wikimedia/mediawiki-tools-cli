package codesearch

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	DefaultFlavour = "search"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func BaseURLForFlavour(flavour string) string {
	return "https://codesearch.wmcloud.org/" + flavour + "/api/v1/search"
}

func NewClient(flavour string) *Client {
	return &Client{
		BaseURL: BaseURLForFlavour(flavour),
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
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
			log.Fatalln(err)
		}

		return fmt.Errorf("unknown error, status code: %d, raw result: %s", res.StatusCode, string(b))
	}

	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		return err
	}

	return nil
}
