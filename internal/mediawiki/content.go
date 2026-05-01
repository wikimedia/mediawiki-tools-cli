package mediawiki

import (
	"encoding/json"
	"fmt"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
)

// WikibasePropertyInput represents a Wikibase property to create
type WikibasePropertyInput struct {
	ID       string
	Label    string
	Datatype string
}

// WikibaseItemInput represents a Wikibase item to create
type WikibaseItemInput struct {
	ID     string
	Label  string
	Claims []WikibaseItemClaimInput
}

// WikibaseItemClaimInput represents a claim in a Wikibase item
type WikibaseItemClaimInput struct {
	Property string
	Value    string
}

// PageInput represents a wiki page to create
type PageInput struct {
	Title   string
	Text    string
	Summary string
}

// CreateWikibaseProperty creates a Wikibase property on the given wiki
func CreateWikibaseProperty(wikiURL, username, password string, prop WikibasePropertyInput) error {
	w, err := mwclient.New(normalizeWikiURL(wikiURL), "mwcli")
	if err != nil {
		return err
	}

	// Login if credentials provided
	if username != "" && password != "" {
		if err := w.Login(username, password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}

	data := map[string]interface{}{
		"labels": map[string]interface{}{
			"en": map[string]string{
				"language": "en",
				"value":    prop.Label,
			},
		},
		"datatype": prop.Datatype,
	}

	dataJSON := mustMarshalJSON(data)

	token, err := w.GetToken(mwclient.CSRFToken)
	if err != nil {
		return fmt.Errorf("unable to obtain csrf token: %w", err)
	}

	editParams := params.Values{
		"action":  "wbeditentity",
		"new":     "property",
		"data":    dataJSON,
		"summary": fmt.Sprintf("Created property %s", prop.ID),
		"token":   token,
	}

	resp, err := w.Post(editParams)
	if err != nil {
		return err
	}

	// Check for edit success (wbeditentity returns entity.id in result)
	id, err := resp.GetString("entity", "id")
	if err != nil || id == "" {
		raw, _ := resp.Marshal()
		return fmt.Errorf("wbeditentity response missing entity.id: %s", string(raw))
	}

	return nil
}

// CreateWikibaseItem creates a Wikibase item on the given wiki
func CreateWikibaseItem(wikiURL, username, password string, item WikibaseItemInput) error {
	w, err := mwclient.New(normalizeWikiURL(wikiURL), "mwcli")
	if err != nil {
		return err
	}

	// Login if credentials provided
	if username != "" && password != "" {
		if err := w.Login(username, password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}

	// Build claims
	claims := []interface{}{}
	for _, claim := range item.Claims {
		claims = append(claims, map[string]interface{}{
			"mainsnak": map[string]interface{}{
				"snaktype": "value",
				"property": claim.Property,
				"datavalue": map[string]string{
					"value": claim.Value,
					"type":  "string",
				},
			},
			"type": "statement",
			"rank": "normal",
		})
	}

	data := map[string]interface{}{
		"labels": map[string]interface{}{
			"en": map[string]string{
				"language": "en",
				"value":    item.Label,
			},
		},
		"claims": claims,
	}

	dataJSON := mustMarshalJSON(data)

	token, err := w.GetToken(mwclient.CSRFToken)
	if err != nil {
		return fmt.Errorf("unable to obtain csrf token: %w", err)
	}

	editParams := params.Values{
		"action":  "wbeditentity",
		"new":     "item",
		"data":    dataJSON,
		"summary": fmt.Sprintf("Created item %s", item.ID),
		"token":   token,
	}

	resp, err := w.Post(editParams)
	if err != nil {
		return err
	}

	// Check for edit success
	id, err := resp.GetString("entity", "id")
	if err != nil || id == "" {
		raw, _ := resp.Marshal()
		return fmt.Errorf("wbeditentity response missing entity.id: %s", string(raw))
	}

	return nil
}

// CreatePage creates or edits a wiki page
func CreatePage(wikiURL, username, password string, page PageInput) error {
	w, err := mwclient.New(normalizeWikiURL(wikiURL), "mwcli")
	if err != nil {
		return err
	}

	// Login if credentials provided
	if username != "" && password != "" {
		if err := w.Login(username, password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}

	token, err := w.GetToken(mwclient.CSRFToken)
	if err != nil {
		return fmt.Errorf("unable to obtain csrf token: %w", err)
	}

	editParams := params.Values{
		"action":  "edit",
		"title":   page.Title,
		"text":    page.Text,
		"summary": page.Summary,
		"token":   token,
	}

	_, err = w.Post(editParams)
	return err
}

// mustMarshalJSON marshals v to JSON or panics
func mustMarshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// normalizeWikiURL ensures the URL is in the format expected by mwclient
func normalizeWikiURL(wikiURL string) string {
	// If it's just a domain, add the API path
	if !stringContains(wikiURL, "/") {
		return "http://" + wikiURL + "/w/api.php"
	}
	if !stringContains(wikiURL, "api.php") {
		if stringEnds(wikiURL, "/") {
			return wikiURL + "w/api.php"
		}
		return wikiURL + "/w/api.php"
	}
	return wikiURL
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringEnds(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
