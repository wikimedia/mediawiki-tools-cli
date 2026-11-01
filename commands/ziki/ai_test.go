package ziki

import (
	"strings"
	"testing"
)

func TestAIBillingEscalatesAndSuccessNeedsReview(t *testing.T) {
	g, out := testGame("1\n1\n1\n")
	g.Player.Morale = 50
	g.intn = func(int) int { return 0 }
	var tokens, cost int64
	for call := 1; call <= 3; call++ {
		previousModel := g.aiModel
		g.useAI()
		if g.aiCalls != call || g.aiTokens <= tokens || g.aiCostCents <= cost || g.aiModel == previousModel {
			t.Fatal("AI model, tokens or bill did not escalate")
		}
		tokens, cost = g.aiTokens, g.aiCostCents
	}
	if !g.tested || g.reviewed || g.merged {
		t.Fatal("AI success must still need human review")
	}
	if !strings.Contains(out.String(), "MIRACLE") || !strings.Contains(out.String(), "running bill") {
		t.Fatal("missing outcome/billing explanation")
	}
	if g.aiTokens != 56000 || g.aiCostCents != 525 {
		t.Fatalf("tokens=%d cost=%d", g.aiTokens, g.aiCostCents)
	}
}

func TestAIHallucinations(t *testing.T) {
	for _, tc := range []struct {
		input     string
		preserved bool
	}{{"1\n", true}, {"2\n", false}} {
		g, out := testGame(tc.input)
		g.Player.Morale = 100
		g.tested = true
		g.reviewed = true
		g.intn = func(int) int { return 1 }
		g.useAI()
		if g.tested != tc.preserved || g.reviewed != tc.preserved {
			t.Fatalf("progress preserved=%t; tested=%t reviewed=%t", tc.preserved, g.tested, g.reviewed)
		}
		if g.aiCalls != 1 || g.aiCostCents != 25 {
			t.Fatal("failed AI call must still be billed")
		}
		if !strings.Contains(out.String(), "invoice survives") {
			t.Fatal("missing invoice outcome")
		}
	}
}

func TestAICancelAndAlreadyMerged(t *testing.T) {
	for _, merged := range []bool{false, true} {
		g, _ := testGame("3\n")
		g.merged = merged
		g.useAI()
		if g.aiCalls != 0 || g.aiTokens != 0 || g.aiCostCents != 0 {
			t.Fatal("cancel/merged should not be billed")
		}
	}
}

func TestAIBillingDoesNotOverflow(t *testing.T) {
	g, _ := testGame(strings.Repeat("1\n", 80))
	g.intn = func(int) int { return 0 }
	for range 80 {
		g.useAI()
		if g.aiTokens < 0 || g.aiCostCents < 0 {
			t.Fatal("counter overflow")
		}
	}
	if g.aiCostCents != maxAIBillCents {
		t.Fatal("fictional bill should saturate at display ceiling")
	}
}

func TestScreenRetainsMapAndActionResults(t *testing.T) {
	g, out := testGame("")
	g.Player = Actor{CurrentLocation: CommandLine, Morale: 100}
	g.clearScreen = true
	g.renderFrame()
	ProcessCommands(g, "map")
	g.renderFrame()
	frame := strings.Split(out.String(), "\x1b[2J\x1b[H")
	if len(frame) != 3 {
		t.Fatalf("clear count = %d", len(frame)-1)
	}
	last := frame[len(frame)-1]
	if !strings.Contains(last, "DESTINATIONS") || !strings.Contains(last, "WMFSlack") || !strings.Contains(last, "COMMANDLINE") {
		t.Fatalf("map lost on redraw: %s", last)
	}
	ProcessCommands(g, "WMF Slack")
	g.renderFrame()
	frame = strings.Split(out.String(), "\x1b[2J\x1b[H")
	if !strings.Contains(frame[len(frame)-1], "ACCESS DENIED") {
		t.Fatal("result lost on redraw")
	}
}

func TestTranscriptHasNoClearEscapes(t *testing.T) {
	g, out := testGame("map\nquit\n")
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[2J") {
		t.Fatal("transcript contains terminal clearing")
	}
}

func TestEveryScreenShowsDestinationsAndCommands(t *testing.T) {
	g, out := testGame("")
	g.Player = Actor{CurrentLocation: CommandLine, Morale: 100}
	g.clearScreen = true
	for _, command := range []string{"look", "IRC", "help", "status"} {
		ProcessCommands(g, command)
		g.renderFrame()
		frames := strings.Split(out.String(), "\x1b[2J\x1b[H")
		last := frames[len(frames)-1]
		for _, text := range []string{"DESTINATIONS", "WMFSlack", "Telegram", "work · ai", "color dark|light|none"} {
			if !strings.Contains(last, text) {
				t.Fatalf("screen after %q missing %q", command, text)
			}
		}
		if strings.Contains(last, "[locked]") {
			t.Fatal("map spoiled the Slack joke")
		}
	}
}

func TestBillingDisplay(t *testing.T) {
	g, out := testGame("1\n")
	g.intn = func(int) int { return 0 }
	g.useAI()
	g.renderAIStatus()
	if strings.Contains(strings.ToLower(out.String()), "fictional") {
		t.Fatal("billing display contains disclaimer")
	}
}

func TestAICannotSkipCampaignTasks(t *testing.T) {
	g, _ := testGame("1\n")
	g.intn = func(int) int { return 0 }
	g.useAI()
	if g.stage != 0 || g.levelDone {
		t.Fatal("AI bypassed ordered tasks")
	}
}

func TestUnreviewedAIFailureRewindsValidation(t *testing.T) {
	g, _ := testGame("2\n")
	g.level = 1
	g.stage = 6
	g.intn = func(int) int { return 1 }
	g.useAI()
	if g.stage != 3 {
		t.Fatalf("AI rework checkpoint = %d, want previous CI task 3", g.stage)
	}
}
