package ziki

import (
	"bufio"
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
	Player      Actor
	ColorScheme string
	input       *bufio.Scanner
	writer      io.Writer
	stopped     bool
	inputErr    error
	intn        func(int) int
}

func (g *Game) Play() error {
	if g.input == nil {
		g.input = bufio.NewScanner(os.Stdin)
	}
	g.ColorScheme = "dark"

	// Should we prompt people for their name instead?
	g.Player = *new(Actor)
	g.Player.Name = "Player" // keep this gender/nationality neutral. We could prompt for this...
	g.Player.Morale = 100
	g.Player.Actions = []int{1, 2, 3, 4, 5, 6}
	g.Player.CurrentLocation = "CommandLine"

	lastLocation := Start

	g.Output(ColorTypes["alert"], Messages["welcome"])
	for !g.stopped {
		g.Output(ColorTypes["normal"], LocationMap[g.Player.CurrentLocation].Description)
		if g.Player.CurrentLocation == lastLocation {
			g.Output(ColorTypes["error"], "You haven't gone anywhere. Type 'help' for available commands.")
		} else {
			lastLocation = g.Player.CurrentLocation

			// We really shouldn't process an event unless location has changed.
			// Otherwise you can stay in AFK forever and get crazy morale by hitting Return over and over
			// And you can also get a Story event immediately after a CodeReview event without entering
			// a command, which is a little confusing.
			g.ProcessEvents(LocationMap[g.Player.CurrentLocation].Events)
			if g.stopped {
				break
			}
			if g.Player.Morale <= 0 {
				g.Output(ColorTypes["alert"], "\nYou have given up hope on your change. Game over.")
				return nil
			}
			g.Output(ColorTypes["normal"], "\tYou are still working on your change.")
			g.Output(ColorTypes["normal"], "\tMorale: ", g.Player.Morale)
		}

		g.Output(ColorTypes["prompt"], "You can go to these places:")
		for _, loc := range LocationMap[g.Player.CurrentLocation].Transitions {
			g.Outputf(ColorTypes["prompt"], "\t%s", loc)
		}
		text, ok := g.readLine("\n >>> ")
		if !ok {
			break
		}
		ProcessCommands(g, text)
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
	if g.input == nil {
		g.input = bufio.NewScanner(os.Stdin)
	}
	fmt.Fprint(g.outputWriter(), prompt)
	if !g.input.Scan() {
		g.inputErr = g.input.Err()
		g.stopped = true
		return "", false
	}
	text := g.input.Text()
	if strings.EqualFold(strings.TrimSpace(text), "quit") {
		ProcessCommands(g, "quit")
		return "", false
	}
	return text, true
}

func (g *Game) readAction(count int) (int, bool) {
	for {
		text, ok := g.readLine("\n >>> ")
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
