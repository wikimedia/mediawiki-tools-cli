package wiki

import (
	"reflect"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestOAuthRiskPresetsAreCumulativeAndExcludeManualGrants(t *testing.T) {
	state := newOAuthSelection(nil)
	if !reflect.DeepEqual(state.scopes(), []string{"basic"}) {
		t.Fatal("new selection is not basic-only")
	}
	for level := 1; level <= 3; level++ {
		state.change(terminal.KeyArrowRight)
		for _, grant := range state.grants {
			risk := oauthRiskLevel(grant.Risk)
			if state.checked[grant.Scope] != (risk > 0 && risk <= level) {
				t.Fatalf("incorrect %s selection at level %d", grant.Scope, level)
			}
		}
		if state.label() != oauthPresetLabels[level] {
			t.Fatal("preset label does not match selected grants")
		}
	}
	state.change(terminal.KeyArrowRight)
	if state.level != 3 {
		t.Fatal("preset wrapped past security")
	}
	for level := 2; level >= 0; level-- {
		state.change(terminal.KeyArrowLeft)
		if state.label() != oauthPresetLabels[level] {
			t.Fatal("left did not reduce selection")
		}
	}
	state.change(terminal.KeyArrowLeft)
	if !reflect.DeepEqual(state.scopes(), []string{"basic"}) {
		t.Fatal("left wrapped or removed basic")
	}
}

func TestOAuthCustomSelectionsSurviveReauthorizationAndPresets(t *testing.T) {
	defaults := []string{"basic", "oauthmanageownclient", "future-grant", "editpage"}
	state := newOAuthSelection(defaults)
	if state.label() != "Custom" {
		t.Fatal("custom grants presented as a known risk preset")
	}
	for i := 0; i < 3; i++ {
		state.change(terminal.KeyArrowRight)
	}
	for i := 0; i < 3; i++ {
		state.change(terminal.KeyArrowLeft)
	}
	if !state.checked["oauthmanageownclient"] || !state.checked["future-grant"] || state.checked["editpage"] {
		t.Fatal("preset changed manual-only grants")
	}
	for _, key := range "future-grant" {
		state.change(key)
	}
	view := state.view(8)
	if len(view.Rows) != 1 || view.Rows[0].Risk != "unknown" || view.Rows[0].Group != "Unknown grants" {
		t.Fatal("unknown scope missing or misclassified", view)
	}
	state.change(terminal.KeySpace)
	if state.checked["future-grant"] || state.label() != "Custom" {
		t.Fatal("manual toggle or internal state lost")
	}
	state.change(terminal.KeyEscape)
	if len(state.visible()) != len(state.grants) {
		t.Fatal("filter not cleared")
	}
}

func TestOAuthPickerFilteringDoesNotChangeGlobalPresetMeaning(t *testing.T) {
	state := newOAuthSelection(nil)
	for _, key := range "editpage" {
		state.change(key)
	}
	if len(state.visible()) != 1 {
		t.Fatal("scope search did not filter")
	}
	state.change(terminal.KeySpace)
	if !state.checked["editpage"] || state.label() != "Custom" {
		t.Fatal("individual toggle failed")
	}
	state.change(terminal.KeyArrowRight)
	if !state.checked["uploadfile"] || !state.checked["createaccount"] || state.checked["editprotected"] {
		t.Fatal("risk preset applied only to filtered rows, or included higher risk")
	}
	state.change(terminal.KeyEscape)
	for i := 0; i < len(state.grants)+2; i++ {
		state.change(terminal.KeyArrowDown)
	}
	view := state.view(8)
	focused := 0
	for _, row := range view.Rows {
		if row.Focused {
			focused++
		}
	}
	if focused != 1 || len(view.Rows) > 8 {
		t.Fatal("pagination lost cursor or exceeded page size")
	}
	for _, key := range "no-matching-scope-zz" {
		state.change(key)
	}
	state.change(terminal.KeyArrowDown)
	state.change(terminal.KeySpace)
	if len(state.view(8).Rows) != 0 {
		t.Fatal("empty filter not handled")
	}
}

func TestOAuthGrantMetadataPreservesUpstreamUnknowns(t *testing.T) {
	seen := map[string]bool{}
	for _, grant := range defaultOAuthGrants {
		if seen[grant.Scope] || grant.Scope == "" {
			t.Fatal("duplicate/empty scope", grant.Scope)
		}
		seen[grant.Scope] = true
		if grant.Scope != "basic" && oauthGroupLabels[grant.Group] == "" {
			t.Fatal("unknown functional group", grant)
		}
		switch grant.Scope {
		case "editmywatchlist":
			if grant.Group != "other" || grant.Risk != "unknown" {
				t.Fatal("upstream's missing metadata was guessed")
			}
		case "oauthmanageownclient":
			if grant.Risk != "internal" {
				t.Fatal("OAuth management not classified as internal")
			}
		case "editprotected":
			if grant.Risk != "vandalism" {
				t.Fatal("protected editing risk not preserved")
			}
		}
	}
	state := newOAuthSelection([]string{"basic", "custom\x1b[31m-scope"})
	state.filter = "custom\x1b[31m-scope"
	view := state.view(8)
	if len(view.Rows) != 1 || strings.Contains(view.Rows[0].Label+view.Rows[0].Scope, "\x1b") {
		t.Fatal("stored scope injected terminal control characters")
	}
}
