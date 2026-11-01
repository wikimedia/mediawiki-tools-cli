package ziki

import (
	"fmt"
	"strings"
)

func (g *Game) renderScene() {
	g.Output(ColorTypes["separator"], "\n  ──────────────────────────────────────────────────────────────")
	g.Outputf(ColorTypes["alert"], "  %s", strings.ToUpper(string(g.Player.CurrentLocation)))
	g.Outputf(ColorTypes["normal"], "  Morale [%s] %d/100   •   Turn %d", moraleBar(g.Player.Morale), g.Player.Morale, g.turns)
	g.Outputf(ColorTypes["alert"], "  Level %d/%d: %s", g.level+1, len(campaign), campaign[g.level].Title)
	g.Outputf(ColorTypes["normal"], "  Role: %s · Tasks %d/%d", campaign[g.level].Role, g.stage, len(campaign[g.level].Steps))
	g.renderAIStatus()
	g.Output(ColorTypes["normal"], "\n  ", LocationMap[g.Player.CurrentLocation].Description)
	g.Output(ColorTypes["prompt"], "  ", g.nextStep())
}

func moraleBar(morale int) string {
	filled := max(0, min(10, morale/10))
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}

func (g *Game) renderMap() {
	g.Output(ColorTypes["alert"], "\n  DESTINATIONS")
	for i := 0; i < len(AllLocationNames); i += 3 {
		var row strings.Builder
		row.WriteString("  ")
		for j := i; j < min(i+3, len(AllLocationNames)); j++ {
			name := string(AllLocationNames[j])
			if aliases := locationAliases[AllLocationNames[j]]; len(aliases) > 0 {
				name += "/" + aliases[0]
			}
			fmt.Fprintf(&row, "%-22s", name)
		}
		g.Output(ColorTypes["normal"], strings.TrimRight(row.String(), " "))
	}
}

func (g *Game) nextStep() string {
	if g.levelDone {
		return "Level complete! Type 'next' to take on your next responsibility."
	}
	step := campaign[g.level].Steps[g.stage]
	return fmt.Sprintf("Next: %s → %s", step.At, missionTasks[step.Task].Title)
}

func (g *Game) choose(title string, options ...string) (int, bool) {
	g.Output(ColorTypes["alert"], "\n  ", title)
	for i, option := range options {
		g.Outputf(ColorTypes["normal"], "  %d. %s", i+1, option)
	}
	choice, ok := g.readAction(len(options))
	if ok && g.clearScreen {
		g.messages = nil
	}
	return choice, ok
}

func (g *Game) adjustMorale(delta int) { g.Player.Morale = max(0, min(100, g.Player.Morale+delta)) }

func (g *Game) work() {
	if g.stopped {
		return
	}
	g.turns++
	switch g.Player.CurrentLocation {
	case AFK:
		g.adjustMorale(25)
		g.Output(ColorTypes["prompt"], "  Tea, fresh air and no pings. +25 morale.")
		return
	case WMFSlack:
		g.Output(ColorTypes["error"], "  Access denied.")
		return
	}
	if g.levelDone {
		g.Output(ColorTypes["prompt"], "  Your part is done. Type 'next' for the next level, or 'levels' to see the campaign.")
		return
	}
	step := campaign[g.level].Steps[g.stage]
	if step.At != g.Player.CurrentLocation {
		switch g.Player.CurrentLocation {
		case IRC, Telegram, Discord:
			g.communityWork()
		default:
			g.Outputf(ColorTypes["normal"], "  No task for your role here yet. %s", g.nextStep())
		}
		return
	}
	task := missionTasks[step.Task]
	options := []string{task.Good, task.Bad}
	// Stable per-step ordering keeps tests/replays reproducible and varies answers.
	correct := (g.level + g.stage) % 2
	if correct == 1 {
		options[0], options[1] = options[1], options[0]
	}
	choice, ok := g.choose(task.Title, options...)
	if !ok {
		return
	}
	if choice != correct {
		g.adjustMorale(-12)
		g.Output(ColorTypes["error"], "  That would not resolve this task safely. -12 morale. Check the evidence and retry.")
		return
	}
	g.adjustMorale(4)
	g.Output(ColorTypes["prompt"], "  ", task.Result)
	g.stage++
	if step.Task == "tests" || step.Task == "retest" || step.Task == "branchTests" {
		g.tested = true
	}
	if step.Task == "review" {
		g.reviewed = true
	}
	if step.Task == "submit" {
		g.merged = true
	}
	if g.stage == len(campaign[g.level].Steps) {
		g.levelDone = true
		g.Outputf(ColorTypes["alert"], "\n  LEVEL %d COMPLETE — %s", g.level+1, campaign[g.level].Title)
		if g.level == 0 {
			g.Output(ColorTypes["normal"], "  Your first patch is merged. The deployment team takes it from here. Go enjoy the +2.")
		}
		if g.level < len(campaign)-1 {
			g.Output(ColorTypes["prompt"], "  Type 'next' to accept your next assignment.")
		}
	}
}

func (g *Game) nextLevel() {
	if !g.levelDone {
		g.Output(ColorTypes["error"], "  Finish your current assignment first.")
		return
	}
	if g.level+1 >= len(campaign) {
		return
	}
	g.level++
	g.stage = 0
	g.levelDone = false
	g.tested = false
	g.reviewed = false
	g.merged = false
	g.adjustMorale(30)
	g.Player.CurrentLocation = CommandLine
	g.Outputf(ColorTypes["alert"], "\n  NEW ASSIGNMENT: %s", campaign[g.level].Title)
	g.Output(ColorTypes["normal"], "  ", campaign[g.level].Brief)
}

func (g *Game) showLevels() {
	g.Output(ColorTypes["alert"], "\n  CAMPAIGN — 12 main levels + 4 expert assignments")
	for i, level := range campaign {
		state := "later"
		if i < g.level || i == g.level && g.levelDone {
			state = "complete"
		} else if i == g.level {
			state = "current"
		}
		g.Outputf(ColorTypes["normal"], "  %2d. %-34s %s", i+1, level.Title, state)
	}
}

func (g *Game) communityWork() {
	stories := map[LocationName][]string{
		IRC:      {"A helpful volunteer finds the exact log line. 'Have you checked the timezone?'", "The channel topic links the deployment calendar. Reading it actually helps.", "A maintainer offers a review. Their nickname has been connected since 2008."},
		Telegram: {"Someone forwards a useful reproduction, then forwards it again. You save the original.", "A community member confirms which release branch needs the fix.", "A sticker arrives, followed by the bug report you needed. A fair exchange."},
		Discord:  {"You turn a screenshot of an error into a searchable Phab report. Progress!", "A volunteer helps reproduce the issue on a test wiki.", "The voice channel agrees: canary first, dashboard second, celebration third."},
	}
	items := stories[g.Player.CurrentLocation]
	g.Output(ColorTypes["normal"], "  ", items[g.randomIntn(len(items))])
	g.adjustMorale(8)
}
