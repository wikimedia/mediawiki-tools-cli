package phabricator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// conduitClient wraps the Phabricator Conduit API.
type conduitClient struct {
	baseURL    string
	apiToken   string
	username   string
	cache      *PhabCache
	httpClient *http.Client
}

func newConduitClient(cfg *PhabConfig) *conduitClient {
	var cache *PhabCache
	if cfg.CachePath != "" {
		cache = newPhabCache(cfg.CachePath)
	}
	return &conduitClient{
		baseURL:    cfg.URL,
		apiToken:   cfg.Key,
		username:   cfg.Username,
		cache:      cache,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ---- Internal Conduit HTTP transport ----

type conduitResponse struct {
	Result    json.RawMessage `json:"result"`
	ErrorCode interface{}     `json:"error_code"`
	ErrorInfo interface{}     `json:"error_info"`
}

func (c *conduitClient) post(method string, params map[string]interface{}) (json.RawMessage, error) {
	params["__conduit__"] = map[string]string{"token": c.apiToken}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling params: %w", err)
	}
	formData := url.Values{
		"params": {string(paramsJSON)},
		"output": {"json"},
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/"+method, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "mwcli-phab/1.0 https://gitlab.wikimedia.org/repos/releng/cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", method, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	var cr conduitResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("parsing response from %s: %w", method, err)
	}
	if cr.ErrorCode != nil && cr.ErrorCode != "" && cr.ErrorCode != float64(0) {
		return nil, fmt.Errorf("conduit %s: %v (info: %v)", method, cr.ErrorCode, cr.ErrorInfo)
	}
	return cr.Result, nil
}

// ---- Data types ----

type phabTask struct {
	ID   int    `json:"id"`
	PHID string `json:"phid"`
	Fields struct {
		Name        string `json:"name"`
		Description struct {
			Raw string `json:"raw"`
		} `json:"description"`
		AuthorPHID  string      `json:"authorPHID"`
		OwnerPHID   interface{} `json:"ownerPHID"` // may be null
		Status      struct {
			Value string `json:"value"`
			Name  string `json:"name"`
		} `json:"status"`
		Priority struct {
			Value int    `json:"value"`
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"priority"`
		DateCreated  int64 `json:"dateCreated"`
		DateModified int64 `json:"dateModified"`
	} `json:"fields"`
	Attachments struct {
		Projects struct {
			ProjectPHIDs []string `json:"projectPHIDs"`
		} `json:"projects"`
	} `json:"attachments"`
}

type phabColumn struct {
	ID   int    `json:"id"`
	PHID string `json:"phid"`
	Fields struct {
		Name     string `json:"name"`
		IsHidden bool   `json:"isHidden"`
	} `json:"fields"`
}

type phabUser struct {
	PHID   string `json:"phid"`
	Fields struct {
		Username string `json:"username"`
		RealName string `json:"realName"`
	} `json:"fields"`
}

type phabProject struct {
	PHID   string `json:"phid"`
	Fields struct {
		Name string `json:"name"`
	} `json:"fields"`
}

type phabTransaction struct {
	ID           int    `json:"id"`
	PHID         string `json:"phid"`
	Type         string `json:"type"`
	AuthorPHID   string `json:"authorPHID"`
	DateCreated  int64  `json:"dateCreated"`
	DateModified int64  `json:"dateModified"`
	Comments     []struct {
		ID      int  `json:"id"`
		Version int  `json:"version"`
		Removed bool `json:"removed"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
		DateCreated  int64 `json:"dateCreated"`
		DateModified int64 `json:"dateModified"`
	} `json:"comments"`
}

type searchResult struct {
	Data   json.RawMessage `json:"data"`
	Cursor struct {
		After interface{} `json:"after"`
	} `json:"cursor"`
}

// ---- Task lookups ----

// lookupTaskPHID resolves a task number like "T12345" to its PHID.
func (c *conduitClient) lookupTaskPHID(taskNumber string) (string, error) {
	cacheKey := "task:" + strings.ToUpper(taskNumber)
	if c.cache != nil {
		var phid string
		if c.cache.phids.get(cacheKey, &phid) {
			return phid, nil
		}
	}

	numStr := strings.TrimPrefix(strings.ToUpper(taskNumber), "T")
	id, err := strconv.Atoi(numStr)
	if err != nil {
		return "", fmt.Errorf("invalid task number %q: %w", taskNumber, err)
	}

	result, err := c.post("maniphest.search", map[string]interface{}{
		"constraints": map[string]interface{}{
			"ids": []int{id},
		},
	})
	if err != nil {
		return "", err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return "", err
	}
	var tasks []phabTask
	if err := json.Unmarshal(sr.Data, &tasks); err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		return "", fmt.Errorf("task %s not found", taskNumber)
	}
	phid := tasks[0].PHID
	if c.cache != nil {
		c.cache.phids.set(cacheKey, phid, ttlDefault)
	}
	return phid, nil
}

// getTask fetches full task details for a task number like "T12345".
func (c *conduitClient) getTask(taskNumber string) (*phabTask, error) {
	phid, err := c.lookupTaskPHID(taskNumber)
	if err != nil {
		return nil, err
	}
	return c.getTaskByPHID(phid)
}

func (c *conduitClient) getTaskByPHID(phid string) (*phabTask, error) {
	result, err := c.post("maniphest.search", map[string]interface{}{
		"constraints": map[string]interface{}{
			"phids": []string{phid},
		},
		"attachments": map[string]interface{}{
			"projects": true,
		},
	})
	if err != nil {
		return nil, err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return nil, err
	}
	var tasks []phabTask
	if err := json.Unmarshal(sr.Data, &tasks); err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("task PHID %s not found", phid)
	}
	return &tasks[0], nil
}

// ---- Project lookups ----

// lookupProjectPHID resolves a project name to its PHID.
// The name may include a leading "#" which will be stripped.
func (c *conduitClient) lookupProjectPHID(name string) (string, error) {
	query := strings.TrimPrefix(name, "#")
	cacheKey := "project:" + query
	if c.cache != nil {
		var phid string
		if c.cache.phids.get(cacheKey, &phid) {
			return phid, nil
		}
	}

	result, err := c.post("project.search", map[string]interface{}{
		"constraints": map[string]interface{}{
			"query": query,
		},
	})
	if err != nil {
		return "", err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return "", err
	}
	var projects []phabProject
	if err := json.Unmarshal(sr.Data, &projects); err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", fmt.Errorf("project %q not found", name)
	}
	phid := projects[0].PHID
	if c.cache != nil {
		c.cache.phids.set(cacheKey, phid, ttlDefault)
	}
	return phid, nil
}

// findProjectsByName searches for projects by name, returning {name: phid} map.
func (c *conduitClient) findProjectsByName(query string) (map[string]string, error) {
	result, err := c.post("project.search", map[string]interface{}{
		"constraints": map[string]interface{}{
			"query": query,
		},
	})
	if err != nil {
		return nil, err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return nil, err
	}
	var projects []phabProject
	if err := json.Unmarshal(sr.Data, &projects); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(projects))
	for _, p := range projects {
		out[p.Fields.Name] = p.PHID
	}
	return out, nil
}

// ---- User lookups ----

// getUsers fetches username and real name for a list of user PHIDs.
// Returns map[phid] -> [2]string{username, realName}.
func (c *conduitClient) getUsers(phids []string) (map[string][2]string, error) {
	if len(phids) == 0 {
		return nil, nil
	}

	out := make(map[string][2]string)
	missing := make([]string, 0, len(phids))
	if c.cache != nil {
		for _, phid := range phids {
			var pair [2]string
			if c.cache.phids.get("user:"+phid, &pair) {
				out[phid] = pair
			} else {
				missing = append(missing, phid)
			}
		}
	} else {
		missing = phids
	}
	if len(missing) == 0 {
		return out, nil
	}

	result, err := c.post("user.search", map[string]interface{}{
		"constraints": map[string]interface{}{
			"phids": missing,
		},
	})
	if err != nil {
		return nil, err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return nil, err
	}
	var users []phabUser
	if err := json.Unmarshal(sr.Data, &users); err != nil {
		return nil, err
	}
	for _, u := range users {
		pair := [2]string{u.Fields.Username, u.Fields.RealName}
		out[u.PHID] = pair
		if c.cache != nil {
			c.cache.phids.set("user:"+u.PHID, pair, ttlDefault)
		}
	}
	return out, nil
}

// ---- Project name resolution ----

// getProjects fetches project names for a list of project PHIDs.
// Returns map[phid] -> name.
func (c *conduitClient) getProjects(phids []string) (map[string]string, error) {
	if len(phids) == 0 {
		return nil, nil
	}
	out := make(map[string]string)
	missing := make([]string, 0, len(phids))
	if c.cache != nil {
		for _, phid := range phids {
			var name string
			if c.cache.phids.get("projname:"+phid, &name) {
				out[phid] = name
			} else {
				missing = append(missing, phid)
			}
		}
	} else {
		missing = phids
	}
	if len(missing) == 0 {
		return out, nil
	}

	result, err := c.post("project.search", map[string]interface{}{
		"constraints": map[string]interface{}{
			"phids": missing,
		},
	})
	if err != nil {
		return nil, err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return nil, err
	}
	var projects []phabProject
	if err := json.Unmarshal(sr.Data, &projects); err != nil {
		return nil, err
	}
	for _, p := range projects {
		out[p.PHID] = p.Fields.Name
		if c.cache != nil {
			c.cache.phids.set("projname:"+p.PHID, p.Fields.Name, ttlDefault)
		}
	}
	return out, nil
}

// ---- Column operations ----

// getColumns returns project columns as a map from normalized name to PHID.
func (c *conduitClient) getColumns(projectPHID string) (map[string]string, error) {
	cacheKey := "columns:" + projectPHID
	if c.cache != nil {
		var cols map[string]string
		if c.cache.phids.get(cacheKey, &cols) {
			return cols, nil
		}
	}

	result, err := c.post("project.column.search", map[string]interface{}{
		"constraints": map[string]interface{}{
			"projects": []string{projectPHID},
		},
	})
	if err != nil {
		return nil, err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return nil, err
	}
	var cols []phabColumn
	if err := json.Unmarshal(sr.Data, &cols); err != nil {
		return nil, err
	}

	mapping := make(map[string]string)
	for _, col := range cols {
		if !col.Fields.IsHidden {
			key := normaliseColumnKey(col.Fields.Name)
			mapping[key] = col.PHID
		}
	}
	if c.cache != nil {
		c.cache.phids.set(cacheKey, mapping, ttlColumns)
	}
	return mapping, nil
}

// ---- Task listing ----

// taskSummary is a lightweight task representation for list views.
type taskSummary struct {
	ID           string
	Title        string
	Priority     string
	DateModified int64
}

// listTasksInColumn lists tasks in a project column.
func (c *conduitClient) listTasksInColumn(projectPHID, columnPHID string, limit int) ([]taskSummary, error) {
	var results []taskSummary
	var after interface{}
	pageSize := 100
	if limit > 0 && limit < pageSize {
		pageSize = limit
	}

	for {
		params := map[string]interface{}{
			"limit": pageSize,
			"constraints": map[string]interface{}{
				"projects":    []string{projectPHID},
				"columnPHIDs": []string{columnPHID},
				"statuses":    []string{"open", "stalled", "progress"},
			},
			"order": "updated",
			"attachments": map[string]interface{}{
				"projects": true,
			},
		}
		if after != nil {
			params["after"] = after
		}

		result, err := c.post("maniphest.search", params)
		if err != nil {
			return nil, err
		}
		var sr searchResult
		if err := json.Unmarshal(result, &sr); err != nil {
			return nil, err
		}
		var tasks []phabTask
		if err := json.Unmarshal(sr.Data, &tasks); err != nil {
			return nil, err
		}

		for _, t := range tasks {
			results = append(results, taskSummary{
				ID:           fmt.Sprintf("T%d", t.ID),
				Title:        t.Fields.Name,
				Priority:     t.Fields.Priority.Name,
				DateModified: t.Fields.DateModified,
			})
		}

		if (limit > 0 && len(results) >= limit) || len(tasks) < pageSize || sr.Cursor.After == nil {
			break
		}
		after = sr.Cursor.After
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// listTasksGlobal lists tasks across the whole instance.
// If limit is 0, defaults to 100 to avoid dumping too much output.
func (c *conduitClient) listTasksGlobal(limit int) ([]taskSummary, error) {
	if limit <= 0 {
		limit = 100
	}

	result, err := c.post("maniphest.search", map[string]interface{}{
		"limit": limit,
		"constraints": map[string]interface{}{
			"statuses": []string{"open", "stalled", "progress"},
		},
		"order": "updated",
	})
	if err != nil {
		return nil, err
	}

	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return nil, err
	}

	var tasks []phabTask
	if err := json.Unmarshal(sr.Data, &tasks); err != nil {
		return nil, err
	}

	results := make([]taskSummary, 0, len(tasks))
	for _, t := range tasks {
		results = append(results, taskSummary{
			ID:           fmt.Sprintf("T%d", t.ID),
			Title:        t.Fields.Name,
			Priority:     t.Fields.Priority.Name,
			DateModified: t.Fields.DateModified,
		})
	}

	return results, nil
}

// ---- Transactions ----

// getTransactions fetches all transactions for a task PHID.
func (c *conduitClient) getTransactions(taskPHID string) ([]phabTransaction, error) {
	cacheKey := "tx:" + taskPHID
	if c.cache != nil {
		var cached []phabTransaction
		if c.cache.transactions.get(cacheKey, &cached) {
			return cached, nil
		}
	}

	result, err := c.post("transaction.search", map[string]interface{}{
		"objectIdentifier": taskPHID,
	})
	if err != nil {
		return nil, err
	}
	var sr searchResult
	if err := json.Unmarshal(result, &sr); err != nil {
		return nil, err
	}
	var txs []phabTransaction
	if err := json.Unmarshal(sr.Data, &txs); err != nil {
		return nil, err
	}

	if c.cache != nil {
		c.cache.transactions.set(cacheKey, txs, ttlTransactions)
	}
	return txs, nil
}

// ---- Task edit operations ----

func (c *conduitClient) editTask(taskPHID string, transactions []map[string]interface{}) error {
	_, err := c.post("maniphest.edit", map[string]interface{}{
		"objectIdentifier": taskPHID,
		"transactions":     transactions,
	})
	return err
}

// moveTaskToColumn moves a task to the given column PHID.
func (c *conduitClient) moveTaskToColumn(taskPHID, columnPHID string) error {
	return c.editTask(taskPHID, []map[string]interface{}{
		{"type": "column", "value": []string{columnPHID}},
	})
}

// removeProjectFromTask removes a project tag from a task.
func (c *conduitClient) removeProjectFromTask(taskPHID, projectPHID string) error {
	return c.editTask(taskPHID, []map[string]interface{}{
		{"type": "projects.remove", "value": []string{projectPHID}},
	})
}

// setTaskPriority sets the priority of a task.
func (c *conduitClient) setTaskPriority(taskPHID, priority string) error {
	return c.editTask(taskPHID, []map[string]interface{}{
		{"type": "priority", "value": priority},
	})
}

// setTaskStatus sets the status of a task.
func (c *conduitClient) setTaskStatus(taskPHID, status string) error {
	return c.editTask(taskPHID, []map[string]interface{}{
		{"type": "status", "value": status},
	})
}

// ---- Column name normalization ----

var (
	reOpenParen = regexp.MustCompile(`\(`)
	reNonWord   = regexp.MustCompile(`[^\w-]`)
)

// normaliseColumnKey normalises a column name for consistent lookup.
// Equivalent to Python: re.sub(r'\(', '-', name) then re.sub(r'[^\w-]', '', name).
func normaliseColumnKey(name string) string {
	name = removeEmoji(name)
	name = reOpenParen.ReplaceAllString(name, "-")
	name = reNonWord.ReplaceAllString(name, "")
	return name
}

// removeEmoji strips emoji and other special Unicode characters from s,
// keeping letters, digits, spaces, and common ASCII punctuation.
func removeEmoji(s string) string {
	var b strings.Builder
	for _, r := range s {
		if isEmojiRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isEmojiRune(r rune) bool {
	// Keep basic ASCII and extended Latin
	if r < 0x2000 {
		return false
	}
	// Keep general punctuation / misc technical
	if r >= 0x2000 && r <= 0x20FF {
		return false
	}
	if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) {
		return true
	}
	return false
}
