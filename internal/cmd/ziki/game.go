package ziki

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/fatih/color"
)

// Game top-level structure for the game.
type Game struct {
	Player      Actor
	ColorScheme string
}

var (
	Out *os.File
	In  *os.File
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	Out = os.Stdout
	In = os.Stdin
}

func (g *Game) Play() {
	g.ColorScheme = "dark"

	// Should we prompt people for their name instead?
	g.Player = *new(Actor)
	g.Player.Name = "Player" // keep this gender/nationality neutral. We could prompt for this...
	g.Player.Morale = 100
	g.Player.Actions = []int{1, 2, 3, 4, 5, 6}
	g.Player.CurrentLocation = "CommandLine"

	lastLocation := Start

	g.Output(ColorTypes["alert"], Messages["welcome"])
	for {
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
			if g.Player.Morale <= 0 {
				g.Output(ColorTypes["alert"], "\nYou have given up hope on your change. Game over.")
				return
			}
			g.Output(ColorTypes["normal"], "\tYou are still working on your change.")
			g.Output(ColorTypes["normal"], "\tMorale: ", g.Player.Morale)
		}

		g.Output(ColorTypes["prompt"], "You can go to these places:")
		for _, loc := range LocationMap[g.Player.CurrentLocation].Transitions {
			g.Outputf(ColorTypes["prompt"], "\t%s", loc)
		}
		cmd := UserInputln()
		ProcessCommands(g, cmd)
	}
}

func (g *Game) ProcessEvents(events []string) {
	for _, evtName := range events {
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
		_, _ = fmt.Fprintln(Out, s)
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
		_, _ = fmt.Fprintln(Out, col(s))
	}
}

func UserInput(i *int) {
	_, _ = fmt.Fscan(In, i)
}

func UserInputln() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n >>> ")
	text, _ := reader.ReadString('\n')
	return text
}

func UserInputContinue() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n Press return to proceed to the next patchset")
	text, _ := reader.ReadString('\n')
	return text
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
