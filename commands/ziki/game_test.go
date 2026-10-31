package ziki

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func testGame(input string) (*Game, *bytes.Buffer) {
	var out bytes.Buffer
	return &Game{input: bufio.NewScanner(strings.NewReader(input)), writer: &out, ColorScheme: "none"}, &out
}

func TestQuitReturnsThroughCobra(t *testing.T) {
	for _, input := range []string{"quit\n", "quit", "help\ncolor none\nquit\n"} {
		t.Run(fmt.Sprintf("input_%q", input), func(t *testing.T) {
			cmd := Cmd()
			var out bytes.Buffer
			cmd.SetIn(strings.NewReader(input))
			cmd.SetOut(&out)
			cmd.SetArgs(nil)
			returned := false
			cmd.PostRun = func(_ *cobra.Command, _ []string) { returned = true }
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !returned {
				t.Fatal("command did not return through Cobra")
			}
			if strings.Count(out.String(), "Goodbye...") != 1 {
				t.Fatalf("output = %q", out.String())
			}
		})
	}
}

func TestGameEOF(t *testing.T) {
	for _, input := range []string{"", "help\n", "color none"} {
		g, _ := testGame(input)
		if err := g.Play(); err != nil {
			t.Fatal(err)
		}
		if !g.stopped {
			t.Fatal("EOF did not stop game")
		}
	}
}

type brokenInput struct{}

func (brokenInput) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestGameInputError(t *testing.T) {
	g, out := testGame("")
	g.input = bufio.NewScanner(brokenInput{})
	if err := g.Play(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("input error = %v; output = %q", err, out.String())
	}
}

func TestSharedInputAcrossPrompts(t *testing.T) {
	g, out := testGame("nonsense\n0\n-1\n3\n1\n\n2\nquit\n")
	if action, ok := g.readAction(2); !ok || action != 0 {
		t.Fatalf("first action = %d, %t", action, ok)
	}
	if strings.Count(out.String(), "Choose an action from 1 to 2.") != 4 {
		t.Fatalf("invalid choices were not retried: %q", out.String())
	}
	if line, ok := g.readLine("continue"); !ok || line != "" {
		t.Fatalf("continue = %q, %t", line, ok)
	}
	if action, ok := g.readAction(2); !ok || action != 1 {
		t.Fatalf("second action = %d, %t", action, ok)
	}
	if _, ok := g.readAction(2); ok || !g.stopped {
		t.Fatal("quit did not stop action prompt")
	}
}

func TestReviewInputStopsCleanly(t *testing.T) {
	for _, input := range []string{"", "quit\n", "0\n-1\n2\nwrong\n1\n\nquit\n"} {
		g, _ := testGame(input)
		actors := Actors{{Name: "Player", Morale: 100, Actions: []int{1}}, {Name: "Reviewer", Npc: true, Morale: 100, Actions: []int{1}}}
		runReview(g, actors)
		if !g.stopped {
			t.Fatal("review should stop at quit or EOF")
		}
	}
}

func TestActorInvalidChoices(t *testing.T) {
	for _, actions := range [][]int{nil, {1}, {1, 2, 3}} {
		actor := Actor{Actions: actions}
		for _, choice := range []int{-10, -1, len(actions), len(actions) + 1} {
			effect, name := actor.Act(choice)
			if effect != 0 || name != "no action" {
				t.Fatalf("invalid choice %d returned %d %q", choice, effect, name)
			}
		}
	}
	actor := Actor{Actions: []int{1}}
	if _, name := actor.Act(0); name != Actions[1].Name {
		t.Fatalf("valid choice = %q", name)
	}
}

func TestEventProbability(t *testing.T) {
	for _, chance := range []int{0, 25, 100} {
		total := 0
		for roll := 0; roll < 100; roll++ {
			g, _ := testGame("")
			g.intn = func(n int) int { return roll }
			event := Event{Type: "Story", Chance: chance, Morale: 1}
			total += event.ProcessEvent(g)
		}
		if total != chance {
			t.Errorf("chance %d triggered %d of 100 times", chance, total)
		}
	}
}

func TestEveryReviewerCanBeSelected(t *testing.T) {
	for index, reviewer := range Reviewers {
		t.Run(reviewer.Name, func(t *testing.T) {
			g, out := testGame("")
			g.Player = Actor{Name: "Player", Morale: 100, Actions: []int{1}}
			g.intn = func(n int) int {
				if n == 100 {
					return 0
				}
				if n != len(Reviewers) {
					t.Fatalf("reviewer random bound = %d", n)
				}
				return index
			}
			event := Event{Type: "CodeReview", Chance: 100}
			event.ProcessEvent(g)
			if !strings.Contains(out.String(), "A "+reviewer.Name+" reviews your code.") {
				t.Fatalf("reviewer not selected: %q", out.String())
			}
		})
	}
}
