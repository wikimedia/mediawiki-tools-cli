package ziki

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// Game top-level structure for the game.
type Game struct {
	Player          Actor
	ColorScheme     string
	input           *bufio.Scanner
	readInteractive func(string) (string, error)
	writer          io.Writer
	stopped         bool
	inputErr        error
	intn            func(int) int
	turns           int
	level           int
	stage           int
	levelDone       bool
	tested          bool
	reviewed        bool
	merged          bool
	clearScreen     bool
	recording       bool
	messages        []gameMessage
	aiCalls         int
	aiTokens        int64
	aiCostCents     int64
	aiModel         string
}

func (g *Game) Play() error {
	if g.input == nil {
		g.input = bufio.NewScanner(os.Stdin)
	}
	if g.ColorScheme == "" {
		g.ColorScheme = "dark"
	}

	// Should we prompt people for their name instead?
	g.Player = *new(Actor)
	g.Player.Name = "Player" // keep this gender/nationality neutral. We could prompt for this...
	g.Player.Morale = 100
	g.Player.Actions = []int{1, 2, 3, 4, 5, 6}
	g.Player.CurrentLocation = "CommandLine"

	g.Output(ColorTypes["alert"], "\n  Z I K I   /   THE SEARCH FOR MERGE")
	g.Output(ColorTypes["separator"], "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	g.Output(ColorTypes["normal"], "  From first patch to incident commander. One assignment at a time.")
	g.Output(ColorTypes["normal"], "  ", campaign[g.level].Brief)
	g.Output(ColorTypes["prompt"], "  Type a destination to jump in, or 'help' for the field guide.")
	for !g.stopped {
		g.renderFrame()
		text, ok := g.readLine("\n  ziki › ")
		if !ok {
			break
		}
		ProcessCommands(g, text)
		if g.Player.Morale <= 0 && !g.stopped {
			g.Output(ColorTypes["error"], "\n  OUT OF SPOONS. Your patch can wait. You log off for today.")
			g.stopped = true
		}
		if g.levelDone && g.level == len(campaign)-1 && !g.stopped {
			g.Output(ColorTypes["alert"], "\n  CAMPAIGN COMPLETE. The wikis are healthy. You have earned a very long tea break.")
			g.Outputf(ColorTypes["normal"], "  %d turns · %d AI calls · final bill %s", g.turns, g.aiCalls, dollars(g.aiCostCents))
			g.stopped = true
		}
	}

	return g.inputErr
}

func (g *Game) ProcessEvents(events []string) {
	for _, evtName := range events {
		if g.stopped {
			break
		}
		g.Player.Morale += Events[evtName].ProcessEvent(g)
	}
}

func (g *Game) Outputf(c string, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	g.Output(c, s)
}

func (g *Game) Output(c string, args ...interface{}) {
	s := fmt.Sprint(args...)
	if g.recording && g.clearScreen {
		g.messages = append(g.messages, gameMessage{color: c, text: s})
	}

	if g.ColorScheme == "none" {
		_, _ = fmt.Fprintln(g.outputWriter(), s)
	} else {
		col := color.BlackString

		if g.ColorScheme == "dark" {
			col = color.WhiteString
		}
		switch c {
		case "green":
			col = color.GreenString
		case "red":
			if g.ColorScheme == "dark" {
				col = color.HiMagentaString
			} else {
				col = color.RedString
			}
		case "blue":
			if g.ColorScheme == "dark" {
				col = color.CyanString
			} else {
				col = color.BlueString
			}
		case "yellow":
			col = color.YellowString
		}
		_, _ = fmt.Fprintln(g.outputWriter(), col(s))
	}
}

func (g *Game) outputWriter() io.Writer {
	if g.writer == nil {
		return os.Stdout
	}
	return g.writer
}

// All prompts share one scanner so buffered input is never discarded.
func (g *Game) readLine(prompt string) (string, bool) {
	if g.stopped {
		return "", false
	}

	var text string
	if g.readInteractive != nil {
		var err error
		text, err = g.readInteractive(prompt)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				g.inputErr = err
			}
			g.stopped = true
			return "", false
		}
	} else {
		if g.input == nil {
			g.input = bufio.NewScanner(os.Stdin)
		}
		fmt.Fprint(g.outputWriter(), prompt)
		if !g.input.Scan() {
			g.inputErr = g.input.Err()
			g.stopped = true
			return "", false
		}
		text = g.input.Text()
	}

	if strings.EqualFold(strings.TrimSpace(text), "quit") {
		ProcessCommands(g, "quit")
		return "", false
	}
	return text, true
}

func (g *Game) readAction(count int) (int, bool) {
	for {
		text, ok := g.readLine("\n  choice › ")
		if !ok {
			return 0, false
		}
		choice, err := strconv.Atoi(strings.TrimSpace(text))
		if err == nil && choice >= 1 && choice <= count {
			return choice - 1, true
		}
		g.Output(ColorTypes["error"], fmt.Sprintf("Choose an action from 1 to %d.", count))
	}
}

func (g *Game) randomIntn(n int) int {
	if g.intn != nil {
		return g.intn(n)
	}
	return rand.Intn(n) // #nosec G404 -- randomness is only used for game events.
}

func (g *Game) setColorScheme(color string) {
	switch color {
	case "dark":
		g.ColorScheme = "dark"
	case "light":
		g.ColorScheme = "light"
	case "none":
		g.ColorScheme = "none"
	default:
		g.Output(ColorTypes["error"], "Unrecognized color scheme.")
	}
}
