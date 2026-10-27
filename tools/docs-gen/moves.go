package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
)

type documentationMove struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Blocked string `json:"blocked,omitempty"`
}

var plainDocumentationRedirect = regexp.MustCompile(`(?i)^#redirect\s*\[\[([^\]|#]+)\]\]$`)

func documentationRedirect(text string) string {
	match := plainDocumentationRedirect.FindStringSubmatch(strings.TrimSpace(documentationBody(text)))
	if match == nil {
		return ""
	}
	return normalizedWikiTitle(match[1])
}

func samePublicationText(before, after string) bool {
	if normalize(before) == normalize(after) {
		return true
	}
	target := documentationRedirect(before)
	return target != "" && target == documentationRedirect(after)
}

func readProposals(documents []document) (map[string]string, error) {
	proposed := map[string]string{}
	for _, doc := range documents {
		data, err := os.ReadFile(doc.Wikitext)
		if err != nil {
			return nil, err
		}
		proposed[doc.Title] = string(data)
	}
	return proposed, nil
}

// Move relationships come from the generator, never from arbitrary wiki links.
// Keep completed manual moves out of the plan, and refuse to split or overwrite
// histories when both titles already contain independent content.
func planDocumentationMoves(documents []document, proposed map[string]string, state review) ([]documentationMove, error) {
	var moves []documentationMove
	claimed := map[string]bool{}
	for _, doc := range documents {
		if doc.MoveFrom == "" {
			continue
		}
		from, to := normalizedWikiTitle(doc.MoveFrom), normalizedWikiTitle(doc.Title)
		if from == to || claimed[from] || claimed[to] || documentationRedirect(proposed[doc.MoveFrom]) != to {
			return nil, fmt.Errorf("invalid or ambiguous documentation move %s → %s", doc.MoveFrom, doc.Title)
		}
		claimed[from], claimed[to] = true, true
		source, sourceFound := state.Pages[doc.MoveFrom]
		target, targetFound := state.Pages[doc.Title]
		if !sourceFound || !targetFound {
			return nil, fmt.Errorf("missing documentation move snapshot for %s → %s", doc.MoveFrom, doc.Title)
		}
		if source.Missing {
			continue
		}
		alreadyRedirects := documentationRedirect(source.Text) == to
		if alreadyRedirects && !target.Missing {
			continue
		}
		move := documentationMove{From: doc.MoveFrom, To: doc.Title}
		switch {
		case alreadyRedirects:
			move.Blocked = "source already redirects to a missing destination; inspect the wiki before publishing"
		case !target.Missing:
			move.Blocked = "destination already exists and source is not its redirect; reconcile the page histories manually"
		case source.PageID <= 0:
			move.Blocked = "source page ID is unavailable"
		case source.Translation || target.Translation:
			move.Blocked = "translation pages cannot be moved by the documentation publisher"
		}
		moves = append(moves, move)
	}
	return moves, nil
}

func documentationMoveFor(moves []documentationMove, title string) *documentationMove {
	for _, move := range moves {
		if move.From == title || move.To == title {
			return &move
		}
	}
	return nil
}

func publicationSafe(config configuration, title string, proposed map[string]string, state review) bool {
	desired := proposed[title]
	if move := documentationMoveFor(state.Moves, title); move != nil {
		if move.Blocked != "" {
			return false
		}
		title, desired = move.From, proposed[move.To]
	}
	current := state.Pages[title]
	var baseline *snapshot
	if value, ok := state.Baselines[title]; ok {
		baseline = &value
	}
	return !current.Translation && safeToPublish(config, title, current, desired, baseline)
}

func sameWikiSnapshot(before, after snapshot) bool {
	return before.Missing == after.Missing && before.PageID == after.PageID && before.Revision == after.Revision && before.Text == after.Text && before.Translation == after.Translation
}

func applyDocumentationMoves(client *mwclient.Client, state *review, moves []documentationMove, summary string) error {
	if len(moves) == 0 {
		return nil
	}
	var titles []string
	for _, move := range moves {
		if move.Blocked != "" {
			return fmt.Errorf("move %s → %s blocked: %s", move.From, move.To, move.Blocked)
		}
		titles = append(titles, move.From, move.To)
	}
	// Check the complete move set before the first write, then recheck each pair
	// immediately before moving. Native move has no baserevid parameter.
	fresh, err := fetchPages(client, titles)
	if err != nil {
		return err
	}
	for _, title := range titles {
		if !sameWikiSnapshot(state.Pages[title], fresh.Pages[title]) {
			return fmt.Errorf("%s changed on the wiki; refresh and review before moving pages", title)
		}
	}
	token, err := client.GetToken(mwclient.CSRFToken)
	if err != nil {
		return err
	}
	for _, move := range moves {
		pair, err := fetchPages(client, []string{move.From, move.To})
		if err != nil {
			return err
		}
		source := state.Pages[move.From]
		if source.PageID <= 0 || source.Missing || !pair.Pages[move.To].Missing || !sameWikiSnapshot(source, pair.Pages[move.From]) {
			return fmt.Errorf("move %s → %s changed since review; inspect any completed moves before retrying", move.From, move.To)
		}
		response, err := client.Post(params.Values{"action": "move", "formatversion": "2", "fromid": strconv.FormatInt(source.PageID, 10), "to": move.To, "reason": summary, "token": token, "watchlist": "nochange"})
		if err != nil {
			return fmt.Errorf("moving %s → %s: %w; inspect any completed moves before retrying", move.From, move.To, err)
		}
		data, err := response.Marshal()
		if err != nil {
			return err
		}
		var result struct {
			Move struct {
				From            string `json:"from"`
				To              string `json:"to"`
				RedirectCreated bool   `json:"redirectcreated"`
			} `json:"move"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		if !result.Move.RedirectCreated || normalizedWikiTitle(result.Move.From) != normalizedWikiTitle(move.From) || normalizedWikiTitle(result.Move.To) != normalizedWikiTitle(move.To) {
			return fmt.Errorf("unexpected move result for %s; inspect the wiki before retrying", move.From)
		}
		after, err := fetchPages(client, []string{move.From, move.To})
		if err != nil {
			return err
		}
		old, destination := after.Pages[move.From], after.Pages[move.To]
		if old.Missing || destination.Missing || old.Translation || destination.Translation || destination.PageID != source.PageID || normalize(destination.Text) != normalize(source.Text) || documentationRedirect(old.Text) != normalizedWikiTitle(move.To) {
			return fmt.Errorf("move %s → %s completed but its content or redirect changed; refresh before updating content", move.From, move.To)
		}
		state.Pages[move.From], state.Pages[move.To] = old, destination
		fmt.Printf("Moved %s → %s (history preserved; redirect left)\n", move.From, move.To)
	}
	return nil
}
