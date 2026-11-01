package ziki

import (
	"fmt"
	"strings"
	"testing"
)

func levelScript(index int) string {
	var script strings.Builder
	for i, step := range campaign[index].Steps {
		fmt.Fprintf(&script, "%s\n%d\n", step.At, (index+i)%2+1)
	}
	return script.String()
}

func TestEveryLevelCanBeCompleted(t *testing.T) {
	for index, level := range campaign {
		t.Run(level.Title, func(t *testing.T) {
			g, out := testGame(levelScript(index))
			g.level = index
			if err := g.Play(); err != nil {
				t.Fatal(err)
			}
			if !g.levelDone || g.stage != len(level.Steps) {
				t.Fatalf("level stuck at %d: %s", g.stage, out.String())
			}
			if !strings.Contains(out.String(), "LEVEL ") || !strings.Contains(out.String(), "COMPLETE") {
				t.Fatal("missing level ending")
			}
		})
	}
}

func TestWholeCampaignCanBeWon(t *testing.T) {
	var script strings.Builder
	for index := range campaign {
		script.WriteString(levelScript(index))
		if index < len(campaign)-1 {
			script.WriteString("next\n")
		}
	}
	g, out := testGame(script.String())
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	if g.level != len(campaign)-1 || !g.levelDone || !strings.Contains(out.String(), "CAMPAIGN COMPLETE") {
		t.Fatalf("campaign stopped at %d/%d: %s", g.level, g.stage, out.String())
	}
}

func TestCampaignDefinitions(t *testing.T) {
	if len(campaign) != 16 {
		t.Fatal("expected 12 main and 4 expert levels")
	}
	used := map[LocationName]bool{}
	for i, level := range campaign {
		if level.Title == "" || level.Role == "" || level.Brief == "" || len(level.Steps) == 0 {
			t.Fatalf("incomplete level %d", i)
		}
		for _, step := range level.Steps {
			if LocationMap[step.At] == nil || step.At == WMFSlack {
				t.Fatalf("invalid destination in level %d: %s", i, step.At)
			}
			task, ok := missionTasks[step.Task]
			if !ok || task.Title == "" || task.Good == "" || task.Bad == "" || task.Result == "" {
				t.Fatalf("invalid task %q", step.Task)
			}
			used[step.At] = true
		}
		if i >= 12 && len(level.Steps) < 12 {
			t.Fatal("expert level needs a longer loop")
		}
	}
	for _, name := range []LocationName{Toolforge, CloudVPS, Wikitech, Grafana, Logstash, Patchdemo, Commons, Translatewiki, BetaCluster, Codesearch, Deployments, SAL} {
		if !used[name] {
			t.Fatalf("new destination %s has no mission", name)
		}
	}
}

func TestOrderAndRoleGates(t *testing.T) {
	g, out := testGame("")
	g.Player = Actor{CurrentLocation: CommandLine, Morale: 100}
	for _, place := range []LocationName{Production, Backports, Gerrit} {
		g.travel(string(place))
		if g.stage != 0 {
			t.Fatal("out-of-order task advanced mission")
		}
	}
	g.nextLevel()
	if g.level != 0 {
		t.Fatal("skipped unfinished level")
	}
	if !strings.Contains(out.String(), "No task for your role") {
		t.Fatal("missing role explanation")
	}
}

func TestWrongChoiceRetriesAndNextResetsProgress(t *testing.T) {
	g, _ := testGame("2\n1\n2\n1\n")
	g.Player = Actor{CurrentLocation: CI, Morale: 100}
	g.work()
	if g.stage != 0 || g.Player.Morale != 88 {
		t.Fatal("wrong decision should cost morale without advancing")
	}
	g.work()
	g.Player.CurrentLocation = Gerrit
	g.work()
	g.work()
	if !g.levelDone {
		t.Fatal("first level did not complete")
	}
	g.aiCalls = 3
	g.aiCostCents = 525
	g.nextLevel()
	if g.level != 1 || g.stage != 0 || g.levelDone || g.merged || g.Player.CurrentLocation != CommandLine || g.aiCostCents != 525 {
		t.Fatal("next did not reset tasks while keeping campaign bill")
	}
}

func TestRepeatedDestinationRequiresEveryVisit(t *testing.T) {
	// Expert failed canary: observation, rollback, backport fix and retests are separate steps.
	index := 13
	g, _ := testGame("1\n")
	g.level = index
	g.stage = 6
	g.Player = Actor{CurrentLocation: Production, Morale: 100}
	before := g.stage
	g.work()
	if g.stage != before {
		t.Fatal("skipped investigation to rollback")
	}
}

func TestSlackNeverOpens(t *testing.T) {
	for _, origin := range AllLocationNames {
		if LocationMap[origin].CanGoTo(WMFSlack) {
			t.Fatalf("Slack reachable from %s", origin)
		}
	}
	for _, destination := range []string{"Slack", "WMFSlack", "wmf slack"} {
		g, out := testGame("")
		g.Player.CurrentLocation = CommandLine
		g.travel(destination)
		if g.Player.CurrentLocation != CommandLine || !strings.Contains(out.String(), "ACCESS DENIED") {
			t.Fatalf("Slack admitted %q", destination)
		}
	}
}

func TestAdventureStopsAtChoiceEOF(t *testing.T) {
	g, _ := testGame("CI\n")
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	if !g.stopped || g.stage != 0 {
		t.Fatal("EOF should stop without progressing")
	}
}

func TestAdventureLosesWhenMoraleRunsOut(t *testing.T) {
	g, out := testGame("CI\n2\n" + strings.Repeat("work\n2\n", 8))
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	if !g.stopped || !strings.Contains(out.String(), "OUT OF SPOONS") {
		t.Fatal("missing loss ending")
	}
}

func TestStatusAndLookDoNotStartActivities(t *testing.T) {
	g, _ := testGame("1\n")
	g.Player = Actor{CurrentLocation: CI, Morale: 100}
	ProcessCommands(g, "look")
	ProcessCommands(g, "status")
	if g.turns != 0 || g.stage != 0 {
		t.Fatal("view started an activity")
	}
	ProcessCommands(g, "work")
	if g.stage != 1 {
		t.Fatal("work did not start activity")
	}
}

func TestLocationsAndCommunityScenes(t *testing.T) {
	for _, name := range AllLocationNames {
		if LocationMap[name] == nil {
			t.Fatalf("missing room %s", name)
		}
		for _, next := range LocationMap[name].Transitions {
			if LocationMap[next] == nil {
				t.Fatalf("unknown transition %s", next)
			}
		}
	}
	for _, name := range []LocationName{IRC, Telegram, Discord} {
		g, out := testGame("")
		g.Player = Actor{CurrentLocation: name, Morale: 50}
		g.intn = func(int) int { return 0 }
		g.work()
		if g.Player.Morale != 58 || out.Len() == 0 {
			t.Fatalf("missing community scene %s", name)
		}
	}
}
