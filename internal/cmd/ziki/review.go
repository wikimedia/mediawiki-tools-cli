package ziki

import (
	"math/rand"
)

func runReview(g *Game, actors Actors) int {
	playerMoraleEffect := 0
	consensus := 0
	round := 1
	action := 0
	for x := 0; x < actors.Len(); x++ {
		actors[x].Output(g, ColorTypes["normal"])
	}

	for {
		g.Output(ColorTypes["normal"], "\nCode Review patchset ", round, " begins...")
		for x := 0; x < actors.Len(); x++ {
			if actors[x].Morale <= 0 {
				continue
			}
			if !actors[x].Npc {
				g.Output(ColorTypes["prompt"], "What do you want to do?")
				for option := 0; option < len(actors[x].Actions); option++ {
					g.Output(ColorTypes["prompt"], "\t", option+1, " - ", Actions[actors[x].Actions[option]].Name)
				}
				UserInput(&action)
				action--
			} else {
				action = rand.Intn(len(actors[x].Actions))
			}
			tgt := selectTarget(actors, x)
			if tgt != -1 {
				effect, actionName := actors[x].Act(action)
				actors[tgt].Morale += effect
				if actors[tgt].Morale < 0 {
					actors[tgt].Morale = 0
				}

				// Remember cumulative effect on player's morale so we can return it later
				if !actors[tgt].Npc {
					playerMoraleEffect += effect
				}

				word := "increasing"
				absEffect := effect
				if effect < 0 {
					consensus -= effect
					absEffect = -effect
					word = "decreasing"
				} else {
					consensus += effect
				}
				if consensus > 100 {
					consensus = 100
				}

				g.Output(ColorTypes["alert"], "\t"+actors[x].Name+" makes ", actionName, ", ", word, " ", actors[tgt].Name, " Morale by ", absEffect, " to ", actors[tgt].Morale, ".")
				g.Output(ColorTypes["normal"], "\tConsensus is at ", consensus, "%")
			}
		}
		if isReviewEnded(actors, consensus) {
			break
		} else {
			round++
		}
		UserInputContinue()
	}

	g.Output(ColorTypes["normal"], "Code Review is over.\n")
	return playerMoraleEffect
}

// This is a little silly, because npcs can only ever target the player,
// and there are currently only ever two participants in a code review.
// But it might get more useful if we expand.
func selectTarget(actors []Actor, selectorIndex int) int {
	y := selectorIndex
	for {
		y = y + 1
		if y >= len(actors) {
			y = 0
		}
		if (actors[y].Npc != actors[selectorIndex].Npc) && actors[y].Morale > 0 {
			return y
		}
		if y == selectorIndex {
			return -1
		}
	}
}

func isReviewEnded(actors []Actor, consensus int) bool {
	if consensus >= 100 {
		return true
	}

	count := make([]int, 2)
	count[0] = 0
	count[1] = 0
	for _, pla := range actors {
		if pla.Morale > 0 {
			if !pla.Npc {
				count[0]++
			} else {
				count[1]++
			}
		}
	}
	if count[0] == 0 || count[1] == 0 {
		return true
	}
	return false
}
