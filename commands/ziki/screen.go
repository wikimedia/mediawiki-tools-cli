package ziki

import "fmt"

type gameMessage struct {
	color string
	text  string
}

// Replay the last action after clearing, so maps, hints and outcomes stay visible.
func (g *Game) renderFrame() {
	messages := g.messages
	g.messages = nil
	g.recording = false
	if g.clearScreen {
		fmt.Fprint(g.outputWriter(), "\x1b[2J\x1b[H")
		g.Output(ColorTypes["alert"], "  Z I K I   /   THE SEARCH FOR MERGE")
	}
	g.renderScene()
	if g.clearScreen && len(messages) > 0 {
		g.Output(ColorTypes["alert"], "\n  LAST ACTION")
		for _, message := range messages {
			g.Output(message.color, message.text)
		}
	}
	g.renderMap()
	g.Output(ColorTypes["separator"], "\n  work · ai · status · map · levels · next · help · quit")
	g.Output(ColorTypes["prompt"], "  color dark|light|none · go <destination> · or type a destination")
	g.recording = true
}
