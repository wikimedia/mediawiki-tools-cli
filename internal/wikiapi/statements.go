package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Native Action API JSON is retained, including unknown read fields. Typed
// validation applies to edited values; --raw-json is the explicit escape hatch.
// Contract: https://doc.wikimedia.org/Wikibase/master/php/docs_topics_json.html
type (
	Snak      map[string]json.RawMessage
	Statement map[string]json.RawMessage
	Reference struct {
		Hash  string            `json:"hash,omitempty"`
		Snaks map[string][]Snak `json:"snaks"`
		Order []string          `json:"snaks-order,omitempty"`
	}
)

type dataValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func rawString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func setJSON(object map[string]json.RawMessage, key string, value interface{}) {
	object[key], _ = json.Marshal(value)
}

func opaqueID(value string) error {
	if strings.TrimSpace(value) == "" || strings.IndexFunc(value, unicode.IsControl) >= 0 || strings.Contains(value, "|") {
		return fmt.Errorf("identifier must be a nonempty opaque value without control characters or pipes")
	}
	return nil
}

func statementProperty(statement Statement) string {
	var snak Snak
	_ = json.Unmarshal(statement["mainsnak"], &snak)
	return rawString(snak["property"])
}

func cloneStatement(statement Statement) Statement {
	data, _ := json.Marshal(statement)
	var cloned Statement
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

// Statement IDs are opaque. Membership is checked in the selected entity's
// claims instead of inferring ownership from a GUID's spelling.
func EntityStatements(entity Entity, property string) ([]Statement, error) {
	if property != "" {
		var err error
		property, err = EntityID("property", property)
		if err != nil {
			return nil, err
		}
	}
	var claims map[string][]Statement
	if raw := entity["claims"]; len(raw) > 0 && string(raw) != "[]" {
		if err := json.Unmarshal(raw, &claims); err != nil {
			return nil, err
		}
	}
	keys := []string{}
	for key := range claims {
		if property == "" || property == key {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	statements := []Statement{}
	for _, key := range keys {
		statements = append(statements, claims[key]...)
	}
	return statements, nil
}

func FindStatement(entity Entity, id string) (Statement, error) {
	if err := opaqueID(id); err != nil {
		return nil, err
	}
	statements, err := EntityStatements(entity, "")
	if err != nil {
		return nil, err
	}
	for _, statement := range statements {
		if rawString(statement["id"]) == id {
			return statement, nil
		}
	}
	return nil, fmt.Errorf("statement %q does not belong to the selected entity", id)
}

type ConflictError struct {
	Resource         string
	Expected, Actual int64
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s is at revision %d, expected %d", e.Resource, e.Actual, e.Expected)
}

func (c *Client) EntityForEdit(ctx context.Context, kind, id, props string, expected int64) (Entity, int64, error) {
	if expected < 0 {
		return nil, 0, fmt.Errorf("--baserevid must be nonnegative")
	}
	entity, err := c.Entity(ctx, kind, id, nil, props, false)
	if err != nil {
		return nil, 0, err
	}
	revision, err := entity.Revision()
	if err != nil {
		return nil, 0, err
	}
	if expected > 0 && expected != revision {
		return nil, 0, &ConflictError{Resource: "entity " + id, Expected: expected, Actual: revision}
	}
	return entity, revision, nil
}

var (
	decimalValue = regexp.MustCompile(`^[+-][0-9]+(?:\.[0-9]+)?$`)
	timeValue    = regexp.MustCompile(`^[+-][0-9]{4,16}-(0[0-9]|1[0-2])-([0-2][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]Z$`)
)

func validDataURI(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

func numberValue(raw json.RawMessage) (float64, error) {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		text = string(raw)
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
		return 0, fmt.Errorf("value must be a finite number")
	}
	return number, nil
}

func ValidateSnak(snak Snak, rawJSON bool) error {
	property, err := EntityID("property", rawString(snak["property"]))
	if err != nil {
		return err
	}
	setJSON(snak, "property", property)
	snakType := rawString(snak["snaktype"])
	if snakType == "novalue" || snakType == "somevalue" {
		if _, exists := snak["datavalue"]; exists {
			return fmt.Errorf("%s snaks must not include datavalue", snakType)
		}
		return nil
	}
	if snakType != "value" {
		return fmt.Errorf("snaktype must be value, novalue or somevalue")
	}
	var value dataValue
	if json.Unmarshal(snak["datavalue"], &value) != nil || value.Type == "" || len(value.Value) == 0 || string(value.Value) == "null" {
		return fmt.Errorf("value snaks require a typed non-null datavalue")
	}
	if rawJSON {
		return nil
	}
	datatype := rawString(snak["datatype"])
	switch datatype {
	case "string", "external-id", "url", "commonsMedia":
		var text string
		if value.Type != "string" || json.Unmarshal(value.Value, &text) != nil || strings.TrimSpace(text) == "" || strings.IndexFunc(text, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s requires a nonempty string datavalue", datatype)
		}
		if datatype == "url" && !validDataURI(text) {
			return fmt.Errorf("typed URL values must use HTTP(S) without embedded credentials")
		}
	case "wikibase-item", "wikibase-property":
		if value.Type != "wikibase-entityid" {
			return fmt.Errorf("%s requires a wikibase-entityid datavalue", datatype)
		}
		var entity struct {
			ID     string          `json:"id"`
			Kind   string          `json:"entity-type"`
			Number json.RawMessage `json:"numeric-id"`
		}
		if json.Unmarshal(value.Value, &entity) != nil {
			return fmt.Errorf("invalid entity-id value")
		}
		kind := strings.TrimPrefix(datatype, "wikibase-")
		id := entity.ID
		if id == "" {
			var n string
			if json.Unmarshal(entity.Number, &n) != nil {
				n = string(entity.Number)
			}
			prefix := "Q"
			if kind == "property" {
				prefix = "P"
			}
			id = prefix + n
		}
		normalized, err := EntityID(kind, id)
		if err != nil || (entity.Kind != "" && entity.Kind != kind) {
			return fmt.Errorf("entity-id does not match %s", datatype)
		}
		if len(entity.Number) > 0 {
			var n string
			if json.Unmarshal(entity.Number, &n) != nil {
				n = string(entity.Number)
			}
			if n != strings.TrimLeft(normalized[1:], "0") {
				return fmt.Errorf("numeric-id disagrees with entity id")
			}
		}
	case "monolingualtext":
		var text struct{ Text, Language string }
		if value.Type != "monolingualtext" || json.Unmarshal(value.Value, &text) != nil || strings.TrimSpace(text.Text) == "" || ValidateLanguages([]string{text.Language}) != nil {
			return fmt.Errorf("monolingualtext requires text and a valid language code")
		}
	case "quantity":
		var quantity struct {
			Amount, Unit           string
			LowerBound, UpperBound *string
		}
		if value.Type != "quantity" || json.Unmarshal(value.Value, &quantity) != nil || !decimalValue.MatchString(quantity.Amount) || (quantity.Unit != "1" && !validDataURI(quantity.Unit)) {
			return fmt.Errorf("quantity requires signed decimal amount and a unit URI (or 1)")
		}
		amount, _ := new(big.Rat).SetString(quantity.Amount)
		for _, bound := range []struct {
			value *string
			lower bool
		}{{quantity.LowerBound, true}, {quantity.UpperBound, false}} {
			if bound.value != nil {
				if !decimalValue.MatchString(*bound.value) {
					return fmt.Errorf("quantity bounds must be signed decimals")
				}
				number, _ := new(big.Rat).SetString(*bound.value)
				if (bound.lower && number.Cmp(amount) > 0) || (!bound.lower && number.Cmp(amount) < 0) {
					return fmt.Errorf("quantity bounds must contain the amount")
				}
			}
		}
	case "time":
		var timestamp struct {
			Time                               string
			Precision, Timezone, Before, After int
			CalendarModel                      string `json:"calendarmodel"`
		}
		if value.Type != "time" || json.Unmarshal(value.Value, &timestamp) != nil || !timeValue.MatchString(timestamp.Time) || timestamp.Precision < 0 || timestamp.Precision > 14 || timestamp.Timezone < -1440 || timestamp.Timezone > 1440 || timestamp.Before < 0 || timestamp.After < 0 || !validDataURI(timestamp.CalendarModel) {
			return fmt.Errorf("time requires signed Wikibase timestamp, precision 0..14 and calendar URI")
		}
		parts := strings.Split(timestamp.Time, "-")
		month, day := parts[len(parts)-2], strings.Split(parts[len(parts)-1], "T")[0]
		if (timestamp.Precision >= 10 && month == "00") || (timestamp.Precision >= 11 && day == "00") {
			return fmt.Errorf("time precision requires nonzero month/day")
		}
		monthNumber, _ := strconv.Atoi(month)
		dayNumber, _ := strconv.Atoi(day)
		days := []int{0, 31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
		if (monthNumber == 0 && dayNumber != 0) || (monthNumber > 0 && dayNumber > days[monthNumber]) {
			return fmt.Errorf("time has an invalid month/day combination")
		}
	case "globe-coordinate":
		var coordinate map[string]json.RawMessage
		if value.Type != "globecoordinate" || json.Unmarshal(value.Value, &coordinate) != nil {
			return fmt.Errorf("coordinates require a globecoordinate object")
		}
		latitude, e1 := numberValue(coordinate["latitude"])
		longitude, e2 := numberValue(coordinate["longitude"])
		precision, e3 := numberValue(coordinate["precision"])
		if e1 != nil || e2 != nil || e3 != nil || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 || precision <= 0 || precision > 360 || !validDataURI(rawString(coordinate["globe"])) {
			return fmt.Errorf("coordinates require bounded latitude/longitude, positive precision and globe URI")
		}
	default:
		return fmt.Errorf("unsupported typed datatype %q; use native JSON with --raw-json", datatype)
	}
	return nil
}

// NewSnak builds supported typed values. Complex values are native value JSON;
// unsupported types require a complete native snak and explicit raw validation.
func NewSnak(property, datatype, snakType, scalar string, valueJSON json.RawMessage) (Snak, error) {
	snak := Snak{}
	setJSON(snak, "property", property)
	setJSON(snak, "snaktype", snakType)
	if datatype != "" {
		setJSON(snak, "datatype", datatype)
	}
	if snakType != "value" {
		if scalar != "" || len(valueJSON) > 0 {
			return nil, fmt.Errorf("novalue/somevalue cannot include a value")
		}
		return snak, ValidateSnak(snak, false)
	}
	var value dataValue
	value.Value = valueJSON
	switch datatype {
	case "string", "external-id", "url", "commonsMedia":
		value.Type = "string"
		if len(valueJSON) == 0 {
			value.Value, _ = json.Marshal(scalar)
		}
	case "wikibase-item", "wikibase-property":
		value.Type = "wikibase-entityid"
		if len(valueJSON) == 0 {
			kind := strings.TrimPrefix(datatype, "wikibase-")
			id, err := EntityID(kind, scalar)
			if err != nil {
				return nil, err
			}
			value.Value, _ = json.Marshal(map[string]string{"entity-type": kind, "id": id})
		}
	case "quantity":
		value.Type = "quantity"
		if len(valueJSON) == 0 {
			if !strings.HasPrefix(scalar, "+") && !strings.HasPrefix(scalar, "-") {
				scalar = "+" + scalar
			}
			value.Value, _ = json.Marshal(map[string]interface{}{"amount": scalar, "unit": "1", "lowerBound": nil, "upperBound": nil})
		}
	case "monolingualtext":
		value.Type = "monolingualtext"
	case "time":
		value.Type = "time"
	case "globe-coordinate":
		value.Type = "globecoordinate"
	default:
		return nil, fmt.Errorf("unsupported typed datatype %q; use --input with --raw-json", datatype)
	}
	if len(value.Value) == 0 {
		return nil, fmt.Errorf("%s requires --value-json with a native value object", datatype)
	}
	setJSON(snak, "datavalue", value)
	return snak, ValidateSnak(snak, false)
}

func ParseReference(data []byte) (Reference, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return Reference{}, fmt.Errorf("reference must be a JSON object")
	}
	for key := range fields {
		if key != "hash" && key != "snaks" && key != "snaks-order" {
			return Reference{}, fmt.Errorf("unsupported reference field %q", key)
		}
	}
	var reference Reference
	err := json.Unmarshal(data, &reference)
	return reference, err
}

func ValidateReference(reference Reference, rawJSON bool) error {
	if len(reference.Snaks) == 0 {
		return fmt.Errorf("reference requires a nonempty snaks map")
	}
	for property, snaks := range reference.Snaks {
		if len(snaks) == 0 {
			return fmt.Errorf("reference property %s has no snaks", property)
		}
		for _, snak := range snaks {
			if err := ValidateSnak(snak, rawJSON); err != nil {
				return err
			}
			if rawString(snak["property"]) != property {
				return fmt.Errorf("reference property key disagrees with snak")
			}
		}
	}
	ordered := map[string]bool{}
	for _, property := range reference.Order {
		if ordered[property] {
			return fmt.Errorf("snaks-order contains duplicate property %s", property)
		}
		ordered[property] = true
		if _, ok := reference.Snaks[property]; !ok {
			return fmt.Errorf("snaks-order references missing property %s", property)
		}
	}
	return nil
}

func ValidateStatement(statement Statement, rawJSON bool) error {
	if statement == nil {
		return fmt.Errorf("statement must be a JSON object")
	}
	if raw, exists := statement["type"]; exists {
		var kind string
		if json.Unmarshal(raw, &kind) != nil || kind != "statement" {
			return fmt.Errorf("statement type must be statement")
		}
	}
	setJSON(statement, "type", "statement")
	if raw, exists := statement["id"]; exists {
		var id string
		if json.Unmarshal(raw, &id) != nil {
			return fmt.Errorf("statement id must be a string")
		}
		if err := opaqueID(id); err != nil {
			return err
		}
	}
	if raw, exists := statement["rank"]; exists {
		var rank string
		if json.Unmarshal(raw, &rank) != nil || rank == "" {
			return fmt.Errorf("rank must be a nonempty string")
		}
	}
	rank := rawString(statement["rank"])
	if rank == "" {
		rank = "normal"
	}
	if rank != "normal" && rank != "preferred" && rank != "deprecated" {
		return fmt.Errorf("rank must be normal, preferred or deprecated")
	}
	setJSON(statement, "rank", rank)
	var snak Snak
	if json.Unmarshal(statement["mainsnak"], &snak) != nil || snak == nil {
		return fmt.Errorf("statement requires mainsnak")
	}
	if err := ValidateSnak(snak, rawJSON); err != nil {
		return err
	}
	setJSON(statement, "mainsnak", snak)
	var qualifiers map[string][]Snak
	if raw := statement["qualifiers"]; len(raw) > 0 && string(raw) != "[]" {
		if err := json.Unmarshal(raw, &qualifiers); err != nil {
			return err
		}
	}
	for property, snaks := range qualifiers {
		if _, err := EntityID("property", property); err != nil {
			return err
		}
		for _, qualifier := range snaks {
			if err := ValidateSnak(qualifier, rawJSON); err != nil {
				return err
			}
			if rawString(qualifier["property"]) != property {
				return fmt.Errorf("qualifier property key disagrees with snak")
			}
		}
	}
	if raw, exists := statement["qualifiers-order"]; exists {
		var order []string
		if json.Unmarshal(raw, &order) != nil {
			return fmt.Errorf("qualifiers-order must be an array")
		}
		seen := map[string]bool{}
		for _, property := range order {
			if seen[property] {
				return fmt.Errorf("duplicate qualifier order property")
			}
			seen[property] = true
			if _, ok := qualifiers[property]; !ok {
				return fmt.Errorf("qualifiers-order references missing property")
			}
		}
	}
	var references []Reference
	if raw := statement["references"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &references); err != nil {
			return err
		}
	}
	for _, reference := range references {
		if err := ValidateReference(reference, rawJSON); err != nil {
			return err
		}
	}
	return nil
}

// WikibaseEditPlan contains public preview data and private validated API fields.
// No credentials or CSRF tokens are present until ExecuteWikibasePlan is called.
type WikibaseEditPlan struct {
	Entity       string      `json:"entity"`
	BaseRevision int64       `json:"baserevid"`
	Action       string      `json:"action"`
	Change       interface{} `json:"change"`
	fields       url.Values
}

func (c *Client) ExecuteWikibasePlan(ctx context.Context, plan *WikibaseEditPlan, options EditOptions) (json.RawMessage, error) {
	if plan == nil || plan.fields == nil || plan.BaseRevision <= 0 {
		return nil, fmt.Errorf("invalid Wikibase edit plan")
	}
	if options.BaseRevision > 0 && options.BaseRevision != plan.BaseRevision {
		return nil, &ConflictError{Resource: "edit plan", Expected: options.BaseRevision, Actual: plan.BaseRevision}
	}
	options.BaseRevision = plan.BaseRevision
	fields, err := c.editFields(options, cloneValues(plan.fields))
	if err != nil {
		return nil, err
	}
	return c.edit(ctx, fields)
}

type StatementChange struct {
	Operation, ID, Rank string
	Statement           Statement
	Snak                Snak
}

func (c *Client) PlanStatement(ctx context.Context, kind, id string, change StatementChange, expected int64, rawJSON bool) (*WikibaseEditPlan, error) {
	id, err := EntityID(kind, id)
	if err != nil {
		return nil, err
	}
	if change.Operation != "add" && change.Operation != "update" && change.Operation != "remove" {
		return nil, fmt.Errorf("unsupported statement operation")
	}
	if change.Operation != "add" {
		if err = opaqueID(change.ID); err != nil {
			return nil, err
		}
	}
	entity, revision, err := c.EntityForEdit(ctx, kind, id, "info|claims", expected)
	if err != nil {
		return nil, err
	}
	var existing Statement
	if change.Operation != "add" {
		existing, err = FindStatement(entity, change.ID)
		if err != nil {
			return nil, err
		}
	}
	plan := &WikibaseEditPlan{Entity: id, BaseRevision: revision}
	if change.Operation == "remove" {
		plan.Action = "wbremoveclaims"
		plan.Change = map[string]string{"statement": change.ID}
		plan.fields = url.Values{"action": {plan.Action}, "claim": {change.ID}}
		return plan, nil
	}
	candidate := cloneStatement(change.Statement)
	if change.Snak != nil {
		if change.Statement != nil {
			return nil, fmt.Errorf("choose typed value or native statement JSON")
		}
		if change.Operation == "update" {
			candidate = cloneStatement(existing)
			if rawString(change.Snak["property"]) != statementProperty(existing) {
				return nil, fmt.Errorf("changing a statement's property requires explicit remove/add")
			}
		} else {
			candidate = Statement{}
		}
		setJSON(candidate, "mainsnak", change.Snak)
	}
	if candidate == nil {
		return nil, fmt.Errorf("provide a statement or typed value")
	}
	if change.Rank != "" {
		setJSON(candidate, "rank", change.Rank)
	}
	if change.Operation == "add" {
		if _, has := candidate["id"]; has {
			return nil, fmt.Errorf("new statement input must not contain id")
		}
	} else {
		if stated := rawString(candidate["id"]); stated != "" && stated != change.ID {
			return nil, fmt.Errorf("statement input id disagrees with selected statement")
		}
		if statementProperty(candidate) != statementProperty(existing) {
			return nil, fmt.Errorf("changing a statement's property requires explicit remove/add")
		}
		setJSON(candidate, "id", change.ID)
	}
	if err = ValidateStatement(candidate, rawJSON); err != nil {
		return nil, err
	}
	plan.Change = candidate
	if change.Operation == "add" {
		plan.Action = "wbeditentity"
		data, _ := json.Marshal(map[string]interface{}{"claims": []Statement{candidate}})
		plan.fields = url.Values{"action": {plan.Action}, "id": {id}, "data": {string(data)}}
	} else {
		plan.Action = "wbsetclaim"
		data, _ := json.Marshal(candidate)
		plan.fields = url.Values{"action": {plan.Action}, "claim": {string(data)}}
	}
	return plan, nil
}

func (c *Client) PlanQualifier(ctx context.Context, kind, id, statementID, operation, hash string, snak Snak, expected int64, rawJSON bool) (*WikibaseEditPlan, error) {
	if operation != "add" && operation != "remove" {
		return nil, fmt.Errorf("unsupported qualifier operation")
	}
	if operation == "add" {
		if err := ValidateSnak(snak, rawJSON); err != nil {
			return nil, err
		}
	} else if err := opaqueID(hash); err != nil {
		return nil, err
	}
	entity, revision, err := c.EntityForEdit(ctx, kind, id, "info|claims", expected)
	if err != nil {
		return nil, err
	}
	statement, err := FindStatement(entity, statementID)
	if err != nil {
		return nil, err
	}
	plan := &WikibaseEditPlan{Entity: strings.ToUpper(id), BaseRevision: revision}
	if operation == "remove" {
		var qualifiers map[string][]Snak
		if raw := statement["qualifiers"]; len(raw) > 0 && string(raw) != "[]" {
			_ = json.Unmarshal(raw, &qualifiers)
		}
		found := false
		for _, snaks := range qualifiers {
			for _, qualifier := range snaks {
				if rawString(qualifier["hash"]) == hash {
					found = true
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("qualifier hash does not belong to this statement")
		}
		plan.Action = "wbremovequalifiers"
		plan.Change = map[string]string{"statement": statementID, "hash": hash}
		plan.fields = url.Values{"action": {plan.Action}, "claim": {statementID}, "qualifiers": {hash}}
	} else {
		plan.Action = "wbsetqualifier"
		plan.Change = snak
		fields := url.Values{"action": {plan.Action}, "claim": {statementID}, "property": {rawString(snak["property"])}, "snaktype": {rawString(snak["snaktype"])}}
		if rawString(snak["snaktype"]) == "value" {
			var value dataValue
			_ = json.Unmarshal(snak["datavalue"], &value)
			fields.Set("value", string(value.Value))
		}
		plan.fields = fields
	}
	return plan, nil
}

func (c *Client) PlanReference(ctx context.Context, kind, id, statementID, operation, hash string, reference Reference, expected int64, rawJSON bool) (*WikibaseEditPlan, error) {
	if operation != "add" && operation != "remove" {
		return nil, fmt.Errorf("unsupported reference operation")
	}
	if operation == "add" {
		if err := ValidateReference(reference, rawJSON); err != nil {
			return nil, err
		}
	} else if err := opaqueID(hash); err != nil {
		return nil, err
	}
	entity, revision, err := c.EntityForEdit(ctx, kind, id, "info|claims", expected)
	if err != nil {
		return nil, err
	}
	statement, err := FindStatement(entity, statementID)
	if err != nil {
		return nil, err
	}
	plan := &WikibaseEditPlan{Entity: strings.ToUpper(id), BaseRevision: revision}
	if operation == "remove" {
		var references []Reference
		_ = json.Unmarshal(statement["references"], &references)
		found := false
		for _, ref := range references {
			if ref.Hash == hash {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("reference hash does not belong to this statement")
		}
		plan.Action = "wbremovereferences"
		plan.Change = map[string]string{"statement": statementID, "hash": hash}
		plan.fields = url.Values{"action": {plan.Action}, "statement": {statementID}, "references": {hash}}
	} else {
		plan.Action = "wbsetreference"
		plan.Change = reference
		snaks, _ := json.Marshal(reference.Snaks)
		plan.fields = url.Values{"action": {plan.Action}, "statement": {statementID}, "snaks": {string(snaks)}}
		if len(reference.Order) > 0 {
			order, _ := json.Marshal(reference.Order)
			plan.fields.Set("snaks-order", string(order))
		}
	}
	return plan, nil
}
