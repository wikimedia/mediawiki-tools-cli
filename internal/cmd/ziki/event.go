package ziki

import (
	"math/rand"
	"time"
)

type Event struct {
	Type        string
	Chance      int
	Description string
	Morale      int
	Evt         string
}

func (e *Event) ProcessEvent(g *Game) int {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	moraleAdjustment := 0
	if e.Chance >= r1.Intn(100) {
		if e.Type == "CodeReview" {
			// Generate reviewer
			reviewer := new(Actor)
			*reviewer = *Reviewers[1+rand.Intn(len(Reviewers)-1)]
			reviewer.Npc = true
			g.Output(ColorTypes["alert"], "\tA "+reviewer.Name+" reviews your code.")
			g.Output(ColorTypes["normal"], "\tCode Review ends when you reach 100% consensus, or one of you reaches 0 Morale.")

			actors := Actors{*reviewer, g.Player}
			moraleAdjustment = runReview(g, actors)
		} else {
			moraleAdjustment = e.Morale
			word := "increasing"
			absMorale := e.Morale
			if e.Morale < 0 {
				word = "decreasing"
				absMorale = -absMorale
			}

			// Warning: if the chained events in data.go are accidentally set up such that event A chains to event B,
			// and event B chains to event A, this will cause infinite AB recursion and crash the game.
			g.Output(ColorTypes["alert"], "\t"+e.Description+" ", word, " your morale by ", absMorale)
			if e.Evt != "" {
				moraleAdjustment += Events[e.Evt].ProcessEvent(g)
			}
		}
	}
	return moraleAdjustment
}
