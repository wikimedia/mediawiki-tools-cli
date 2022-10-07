package ziki

import "math/rand"

// Actor
// Used for both the player and all npcs
// By convention, if a variable name is "player", it refers
// to the main player character and not an npc.
type Actor struct {
	Name    string
	Morale  int
	Actions []int
	Npc     bool

	CurrentLocation LocationName
}

type Actors []Actor

func (a *Actor) Act(actionOption int) (int, string) {
	if actionOption > len(a.Actions) {
		randAction := a.Actions[rand.Intn(len(a.Actions)-1)]
		return Actions[randAction].Use(), "no action"
	}
	action := a.Actions[actionOption]
	return Actions[action].Use(), Actions[action].Name
}

func (slice Actors) Len() int {
	return len(slice)
}

func (a *Actor) Output(g *Game, c string) {
	g.Output(c, "\t", a.Name, " Morale: ", a.Morale)
}
