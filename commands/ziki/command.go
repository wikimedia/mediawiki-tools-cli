package ziki

import "strings"

func ProcessCommands(g *Game, input string) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return
	}
	command := strings.ToLower(tokens[0])
	argument := strings.Join(tokens[1:], " ")
	switch command {
	case "goto", "go":
		g.travel(argument)
	case "next":
		g.nextLevel()
	case "levels":
		g.showLevels()
	case "ai", "llm":
		g.useAI()
	case "work", "act":
		g.work()
	case "look", "status":
		// The main loop redraws the room and quest status.
	case "map", "services":
		g.Output(ColorTypes["prompt"], "  Type any destination below to travel.")
	case "color":
		g.setColorScheme(argument)
	case "help", "?":
		g.Output(ColorTypes["alert"], "\n  FIELD GUIDE")
		g.Output(ColorTypes["normal"], "  <destination> / go <name>   Travel and act (try IRC, CI, Backports)")
		g.Output(ColorTypes["normal"], "  work                       Retry / continue your current activity")
		g.Output(ColorTypes["normal"], "  ai                         Ask an increasingly expensive LLM")
		g.Output(ColorTypes["normal"], "  next / levels              Next assignment / campaign overview")
		g.Output(ColorTypes["normal"], "  map                        Show all destinations")
		g.Output(ColorTypes["normal"], "  status / look              Check your patch and surroundings")
		g.Output(ColorTypes["normal"], "  color dark|light|none       Choose terminal colours")
		g.Output(ColorTypes["normal"], "  quit                       Log off; no progress is saved")
		g.Output(ColorTypes["prompt"], "\n  Follow the Next hint. Each level has its own ordered tasks and return visits.")
		g.Output(ColorTypes["normal"], "  Community channels offer clues and morale. AFK restores morale.")
	case "quit", "exit":
		g.Output(ColorTypes["alert"], "You have abandoned your change. Goodbye...")
		g.stopped = true
	default:
		if _, err := LocationNameFromString(input); err == nil {
			g.travel(input)
		} else {
			g.Output(ColorTypes["error"], "  Unknown command or destination. Try 'map' or 'help'.")
		}
	}
}

func (g *Game) travel(destination string) {
	name, err := LocationNameFromString(destination)
	if err != nil {
		g.Output(ColorTypes["error"], "  ", err)
		return
	}
	if name == WMFSlack {
		g.Output(ColorTypes["error"], "\n  🔒 WMF SLACK — ACCESS DENIED")
		g.Output(ColorTypes["normal"], "  You try your badge. You try a different lanyard. Still no.")
		g.Output(ColorTypes["normal"], "  This door never opens. Try IRC, Telegram or Discord instead.")
		return
	}
	if name != g.Player.CurrentLocation {
		if !LocationMap[g.Player.CurrentLocation].CanGoTo(name) {
			g.Output(ColorTypes["error"], "  You can't get there from here.")
			return
		}
		g.Player.CurrentLocation = name
	}
	// Show the new room before presenting its choices, rather than prompting
	// underneath the previous destination's status panel.
	if g.clearScreen {
		g.renderFrame()
	} else {
		g.Outputf(ColorTypes["alert"], "\n  %s", strings.ToUpper(string(name)))
		g.Output(ColorTypes["normal"], "  ", LocationMap[name].Description)
	}
	g.work()
}
