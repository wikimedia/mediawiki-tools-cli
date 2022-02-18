package ziki

import (
	"os"
	"strings"
)

func ProcessCommands(g *Game, input string) {
	g.Output(ColorTypes["separator"], "======================================================================")
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		g.Output(ColorTypes["error"], "No command received.")
		return
	}
	command := strings.ToLower(tokens[0])
	param1 := ""
	if len(tokens) > 1 {
		param1 = tokens[1]
	}
	switch command {
	case "goto":
		loc := LocationMap[g.Player.CurrentLocation]
		locName, err := FindLocationName(strings.ToLower(param1))
		if err != nil {
			g.Output(ColorTypes["error"], err)
		} else if loc.CanGoTo(strings.ToLower(locName)) {
			g.Player.CurrentLocation = locName
		} else {
			g.Output(ColorTypes["error"], "Can't go to "+param1+" from here.")
		}
	case "color":
		g.setColorScheme(param1)
	case "help":
		g.Output(ColorTypes["alert"], "Commands:")
		g.Output(ColorTypes["alert"], "\tgoto <Location Name> - Move to the new location")
		g.Output(ColorTypes["alert"], "\tcolor <dark|light|none> - Set text colors for a dark/light terminal background (or 	disable colors)")
		g.Output(ColorTypes["alert"], "\thelp View this help screen")
		g.Output(ColorTypes["alert"], "\tquit Abandon your change and exit the game")
		g.Output(ColorTypes["alert"], "\n\n")
	case "quit":
		g.Output(ColorTypes["alert"], "You have abandoned your change. Goodbye...")
		os.Exit(0)
	default:
	}
}
