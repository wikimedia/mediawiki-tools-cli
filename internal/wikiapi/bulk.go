package wikiapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
)

// Bulk is an explicit JSONL write interface. It never consumes page selections
// implicitly, changes a supplied base revision, or retries any failed record.
type (
	BulkOptions struct{ DryRun, StopOnError bool }
	BulkStats   struct{ Processed, Succeeded, Planned, Failed int }
	BulkError   struct{ Failed int }
)

func (e *BulkError) Error() string { return fmt.Sprintf("%d bulk record(s) failed", e.Failed) }

type BulkRecordError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Ambiguous bool   `json:"ambiguous,omitempty"`
}
type BulkResult struct {
	Line      int              `json:"line"`
	RecordID  string           `json:"record_id,omitempty"`
	Operation string           `json:"operation,omitempty"`
	Status    string           `json:"status"`
	Result    json.RawMessage  `json:"result,omitempty"`
	Error     *BulkRecordError `json:"error,omitempty"`
}
type recordMeta struct {
	RecordID     string `json:"record_id,omitempty"`
	Operation    string `json:"operation"`
	Summary      string `json:"summary"`
	BaseRevision int64  `json:"baserevid,omitempty"`
	Bot          bool   `json:"bot,omitempty"`
}

func (m recordMeta) editOptions() EditOptions {
	return EditOptions{Summary: m.Summary, BaseRevision: m.BaseRevision, Bot: m.Bot}
}

func decodeBulk[T any](data []byte) (T, error) {
	var result T
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&result)
	if err == nil {
		var extra interface{}
		if decoder.Decode(&extra) != io.EOF {
			err = fmt.Errorf("expected exactly one JSON object per line")
		}
	}
	return result, err
}

func kindRequired(kind string) error {
	if kind != "item" && kind != "property" {
		return fmt.Errorf("kind must be item or property")
	}
	return nil
}

func (c *Client) RunBulk(ctx context.Context, input io.Reader, output io.Writer, options BulkOptions) (BulkStats, error) {
	stats := BulkStats{}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 8*1024*1024)
	encoder := json.NewEncoder(output)
	line := 0
	for scanner.Scan() {
		line++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		stats.Processed++
		result := c.applyBulkLine(ctx, scanner.Bytes(), line, options.DryRun)
		switch result.Status {
		case "success":
			stats.Succeeded++
		case "planned":
			stats.Planned++
		default:
			stats.Failed++
		}
		if err := encoder.Encode(result); err != nil {
			return stats, err
		}
		if result.Error != nil && options.StopOnError {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		stats.Processed++
		stats.Failed++
		result := BulkResult{Line: line + 1, Status: "error", Error: &BulkRecordError{Code: "input_error", Message: err.Error()}}
		if e := encoder.Encode(result); e != nil {
			return stats, e
		}
	}
	if stats.Failed > 0 {
		return stats, &BulkError{Failed: stats.Failed}
	}
	return stats, nil
}

type bulkWriteObserver struct {
	base      http.RoundTripper
	attempted *bool
}

func (t bulkWriteObserver) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPost {
		*t.attempted = true
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func (c *Client) applyBulkLine(ctx context.Context, data []byte, line int, dryRun bool) BulkResult {
	result := BulkResult{Line: line, Status: "error"}
	var meta recordMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		result.Error = &BulkRecordError{Code: "invalid_record", Message: err.Error()}
		return result
	}
	result.RecordID, result.Operation = meta.RecordID, meta.Operation
	if strings.TrimSpace(meta.Summary) == "" {
		result.Error = &BulkRecordError{Code: "invalid_record", Message: "each record requires an explicit nonempty summary"}
		return result
	}
	creating := meta.Operation == "entity-create"
	if (creating && meta.BaseRevision != 0) || (!creating && meta.Operation != "page-put" && meta.BaseRevision <= 0) {
		result.Error = &BulkRecordError{Code: "invalid_record", Message: "updates require positive baserevid; creates require baserevid 0"}
		return result
	}
	// Track the actual POST boundary, so a failed CSRF GET is not reported as an
	// ambiguous write. The observer wraps the existing installation-scoped client.
	attempted := false
	client := *c
	httpClient := *c.HTTP
	httpClient.Transport = bulkWriteObserver{base: c.HTTP.Transport, attempted: &attempted}
	client.HTTP = &httpClient
	payload, err := client.applyBulkOperation(ctx, data, meta, dryRun)
	if err == nil {
		result.Status = "success"
		if dryRun {
			result.Status = "planned"
		}
		result.Result = payload
		return result
	}
	failure := &BulkRecordError{Code: "operation_failed", Message: err.Error()}
	var validation *bulkValidationError
	var api *APIError
	var conflict *ConflictError
	var httpError *restapi.HTTPError
	switch {
	case errors.As(err, &validation):
		failure.Code = "invalid_record"
	case errors.As(err, &api):
		failure.Code = api.Code
	case errors.As(err, &conflict):
		failure.Code = "conflict"
	case errors.As(err, &httpError):
		failure.Code = fmt.Sprintf("http_%d", httpError.StatusCode)
	}
	failure.Ambiguous = attempted && api == nil && (httpError == nil || httpError.StatusCode >= 500)
	result.Error = failure
	return result
}

type bulkValidationError struct{ err error }

func (e *bulkValidationError) Error() string { return e.err.Error() }
func (e *bulkValidationError) Unwrap() error { return e.err }
func invalidBulk(err error) error {
	if err == nil {
		return nil
	}
	return &bulkValidationError{err}
}
func previewBulk(value interface{}) (json.RawMessage, error) { return json.Marshal(value) }

func (c *Client) applyBulkOperation(ctx context.Context, data []byte, meta recordMeta, dryRun bool) (json.RawMessage, error) {
	options := meta.editOptions()
	switch meta.Operation {
	case "page-put":
		record, err := decodeBulk[struct {
			recordMeta
			Title      string  `json:"title"`
			Text       *string `json:"text"`
			CreateOnly bool    `json:"createonly,omitempty"`
		}](data)
		if err != nil {
			return nil, invalidBulk(err)
		}
		if record.Text == nil {
			return nil, invalidBulk(fmt.Errorf("page-put requires text (empty string is allowed)"))
		}
		plan, err := c.PlanPageEdit(ctx, record.Title, *record.Text, record.BaseRevision, record.CreateOnly)
		if err != nil {
			return nil, err
		}
		if dryRun {
			return previewBulk(plan)
		}
		return c.ExecutePageEdit(ctx, plan, options)
	case "entity-create":
		record, err := decodeBulk[struct {
			recordMeta
			Kind string          `json:"kind"`
			Data json.RawMessage `json:"data"`
		}](data)
		if err != nil {
			return nil, invalidBulk(err)
		}
		if err = kindRequired(record.Kind); err != nil {
			return nil, invalidBulk(err)
		}
		entity, err := ValidateEntityCreate(record.Kind, record.Data)
		if err != nil {
			return nil, invalidBulk(err)
		}
		if dryRun {
			return previewBulk(map[string]interface{}{"action": "wbeditentity", "new": record.Kind, "data": entity})
		}
		return c.CreateEntity(ctx, record.Kind, record.Data, options)
	case "term-change":
		record, err := decodeBulk[struct {
			recordMeta
			Kind     string   `json:"kind"`
			Entity   string   `json:"entity"`
			Term     string   `json:"term"`
			Change   string   `json:"change"`
			Language string   `json:"language"`
			Values   []string `json:"values"`
		}](data)
		if err != nil {
			return nil, invalidBulk(err)
		}
		if err = kindRequired(record.Kind); err != nil {
			return nil, invalidBulk(err)
		}
		if err = ValidateTermChange(record.Term, record.Change, record.Language, record.Values); err != nil {
			return nil, invalidBulk(err)
		}
		_, _, err = c.EntityForEdit(ctx, record.Kind, record.Entity, "info", record.BaseRevision)
		if err != nil {
			return nil, err
		}
		if dryRun {
			return previewBulk(record)
		}
		return c.ChangeTerm(ctx, record.Kind, record.Entity, record.Term, record.Change, record.Language, record.Values, options)
	case "statement-add", "statement-update", "statement-remove":
		record, err := decodeBulk[struct {
			recordMeta
			Kind        string    `json:"kind"`
			Entity      string    `json:"entity"`
			StatementID string    `json:"statement_id,omitempty"`
			Statement   Statement `json:"statement,omitempty"`
			RawJSON     bool      `json:"raw_json,omitempty"`
		}](data)
		if err != nil {
			return nil, invalidBulk(err)
		}
		if err = kindRequired(record.Kind); err != nil {
			return nil, invalidBulk(err)
		}
		operation := strings.TrimPrefix(meta.Operation, "statement-")
		if operation == "remove" && (record.Statement != nil || record.RawJSON) {
			return nil, invalidBulk(fmt.Errorf("statement-remove must not include statement/raw_json"))
		}
		if operation == "add" && record.StatementID != "" {
			return nil, invalidBulk(fmt.Errorf("new statements must not include statement_id"))
		}
		if operation != "remove" {
			if err = ValidateStatement(record.Statement, record.RawJSON); err != nil {
				return nil, invalidBulk(err)
			}
		}
		plan, err := c.PlanStatement(ctx, record.Kind, record.Entity, StatementChange{Operation: operation, ID: record.StatementID, Statement: record.Statement}, record.BaseRevision, record.RawJSON)
		if err != nil {
			return nil, err
		}
		if dryRun {
			return previewBulk(plan)
		}
		return c.ExecuteWikibasePlan(ctx, plan, options)
	case "qualifier-add", "qualifier-remove":
		record, err := decodeBulk[struct {
			recordMeta
			Kind        string `json:"kind"`
			Entity      string `json:"entity"`
			StatementID string `json:"statement_id"`
			Hash        string `json:"hash,omitempty"`
			Snak        Snak   `json:"snak,omitempty"`
			RawJSON     bool   `json:"raw_json,omitempty"`
		}](data)
		if err != nil {
			return nil, invalidBulk(err)
		}
		if err = kindRequired(record.Kind); err != nil {
			return nil, invalidBulk(err)
		}
		operation := strings.TrimPrefix(meta.Operation, "qualifier-")
		if (operation == "add" && record.Hash != "") || (operation == "remove" && (record.Snak != nil || record.RawJSON)) {
			return nil, invalidBulk(fmt.Errorf("qualifier-add takes snak; qualifier-remove takes hash"))
		}
		plan, err := c.PlanQualifier(ctx, record.Kind, record.Entity, record.StatementID, operation, record.Hash, record.Snak, record.BaseRevision, record.RawJSON)
		if err != nil {
			return nil, err
		}
		if dryRun {
			return previewBulk(plan)
		}
		return c.ExecuteWikibasePlan(ctx, plan, options)
	case "reference-add", "reference-remove":
		record, err := decodeBulk[struct {
			recordMeta
			Kind        string     `json:"kind"`
			Entity      string     `json:"entity"`
			StatementID string     `json:"statement_id"`
			Hash        string     `json:"hash,omitempty"`
			Reference   *Reference `json:"reference,omitempty"`
			RawJSON     bool       `json:"raw_json,omitempty"`
		}](data)
		if err != nil {
			return nil, invalidBulk(err)
		}
		if err = kindRequired(record.Kind); err != nil {
			return nil, invalidBulk(err)
		}
		operation := strings.TrimPrefix(meta.Operation, "reference-")
		if (operation == "add" && (record.Reference == nil || record.Hash != "")) || (operation == "remove" && (record.Reference != nil || record.RawJSON)) {
			return nil, invalidBulk(fmt.Errorf("reference-add takes reference; reference-remove takes hash"))
		}
		reference := Reference{}
		if record.Reference != nil {
			reference = *record.Reference
		}
		plan, err := c.PlanReference(ctx, record.Kind, record.Entity, record.StatementID, operation, record.Hash, reference, record.BaseRevision, record.RawJSON)
		if err != nil {
			return nil, err
		}
		if dryRun {
			return previewBulk(plan)
		}
		return c.ExecuteWikibasePlan(ctx, plan, options)
	default:
		return nil, invalidBulk(fmt.Errorf("unsupported bulk operation %q", meta.Operation))
	}
}
