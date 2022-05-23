package toolhub

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Tools struct {
	Count    int         `json:"count"`
	Next     string      `json:"next"`
	Previous interface{} `json:"previous"`
	Results  []Tool      `json:"results"`
}

type Tool struct {
	Name                 string        `json:"name"`
	Title                string        `json:"title"`
	Description          string        `json:"description"`
	URL                  string        `json:"url"`
	Keywords             []string      `json:"keywords"`
	Author               []interface{} `json:"author"`
	Repository           string        `json:"repository"`
	Subtitle             interface{}   `json:"subtitle"`
	OpenhubID            interface{}   `json:"openhub_id"`
	URLAlternates        []interface{} `json:"url_alternates"`
	BotUsername          interface{}   `json:"bot_username"`
	Deprecated           bool          `json:"deprecated"`
	ReplacedBy           interface{}   `json:"replaced_by"`
	Experimental         bool          `json:"experimental"`
	ForWikis             []interface{} `json:"for_wikis"`
	Icon                 interface{}   `json:"icon"`
	License              interface{}   `json:"license"`
	Sponsor              []interface{} `json:"sponsor"`
	AvailableUILanguages []interface{} `json:"available_ui_languages"`
	TechnologyUsed       []interface{} `json:"technology_used"`
	Type                 string        `json:"tool_type"`
	APIURL               interface{}   `json:"api_url"`
	DeveloperDocsURL     []interface{} `json:"developer_docs_url"`
	UserDocsURL          []interface{} `json:"user_docs_url"`
	FeedbackURL          []interface{} `json:"feedback_url"`
	PrivacyPolicyURL     []interface{} `json:"privacy_policy_url"`
	TranslateURL         interface{}   `json:"translate_url"`
	BugtrackerURL        interface{}   `json:"bugtracker_url"`
	Schema               interface{}   `json:"_schema"`
	Language             string        `json:"_language"`
	Origin               string        `json:"origin"`
	CreatedBy            struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	} `json:"created_by"`
	CreatedDate time.Time `json:"created_date"`
	ModifiedBy  struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	} `json:"modified_by"`
	ModifiedDate time.Time `json:"modified_date"`
}

type ToolsOptions struct {
	Limit int `json:"limit"`
	Page  int `json:"page"`
}

func (c *Client) SearchTools(ctx context.Context, searchString string, options *ToolsOptions) (*Tools, error) {
	limit := 1500
	page := 1
	if options != nil {
		limit = options.Limit
		page = options.Page
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/search/tools?q=%s&format=json&page_size=%d&page=%d", c.BaseURL, searchString, limit, page), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	res := Tools{}
	if err := c.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *Client) GetTools(ctx context.Context, options *ToolsOptions) (*Tools, error) {
	limit := 1500
	page := 1
	if options != nil {
		limit = options.Limit
		page = options.Page
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/tools?format=json&page_size=%d&page=%d", c.BaseURL, limit, page), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	res := Tools{}
	if err := c.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *Client) GetTool(ctx context.Context, name string, options *ToolsOptions) (*Tool, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/tools/%s/?format=json", c.BaseURL, name), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	res := Tool{}
	if err := c.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}
