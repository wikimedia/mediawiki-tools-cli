package toolhub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	BaseURLV1 = "https://toolhub.wikimedia.org/api"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient() *Client {
	return &Client{
		BaseURL: BaseURLV1,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func (c *Client) sendRequest(req *http.Request, v interface{}) error {
	req.Header.Set("User-Agent", "mwcli toolhub")
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
