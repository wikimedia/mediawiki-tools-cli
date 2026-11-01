package ziki

import "fmt"

const maxAIBillCents int64 = 99999999999999

func (g *Game) renderAIStatus() {
	if g.aiCalls == 0 {
		g.Output(ColorTypes["normal"], "  AI: not consulted · bill $0.00")
		return
	}
	g.Outputf(ColorTypes["normal"], "  AI: %s", g.aiModel)
	g.Outputf(ColorTypes["normal"], "  Calls %d · Tokens %s · Bill %s", g.aiCalls, commaNumber(g.aiTokens), dollars(g.aiCostCents))
}

func (g *Game) useAI() {
	if g.stopped {
		return
	}
	if g.merged || g.levelDone {
		g.Output(ColorTypes["normal"], "  Your patch has landed. The AI offers to rewrite it in Rust. You close the tab.")
		return
	}
	choice, ok := g.choose("RENT AN INCREASINGLY LARGE LANGUAGE MODEL", "Review its suggested patch and actually run the tests", "Trust the generated patch completely. It sounds very sure.", "Cancel before the billing department notices")
	if !ok || choice == 2 {
		return
	}
	g.turns++
	g.aiCalls++
	tiers := []string{"Mini", "Pro", "Ultra", "Max", "Hyper", "Omni", "Infinite"}
	tier := tiers[min(g.aiCalls-1, len(tiers)-1)]
	g.aiModel = fmt.Sprintf("MergeMind-%d-%s-Thinking", g.aiCalls, tier)
	// Context and price grow together. Bound the fictional counters to avoid overflow.
	exponent := min(g.aiCalls-1, 20)
	tokens := int64(8000) << exponent
	charge := min(int64(25)<<(2*exponent), maxAIBillCents)
	g.aiTokens = min(g.aiTokens+tokens, maxAIBillCents)
	g.aiCostCents = min(g.aiCostCents+charge, maxAIBillCents)
	g.Outputf(ColorTypes["alert"], "\n  %s has finished thinking about thinking.", g.aiModel)
	g.Outputf(ColorTypes["normal"], "  +%s tokens · +%s · running bill %s", commaNumber(tokens), dollars(charge), dollars(g.aiCostCents))
	useful := g.randomIntn(2) == 0
	if useful {
		g.tested = true
		g.adjustMorale(12)
		g.Output(ColorTypes["prompt"], "  MIRACLE: a tiny, correct patch and a regression test that actually fails before the fix.")
		g.Output(ColorTypes["normal"], "  Useful material for your assignment. It does not replace the required checks or human decisions.")
		return
	}
	if choice == 0 {
		g.adjustMorale(-8)
		g.Output(ColorTypes["error"], "  HALLUCINATION: it imported wikimedia/fix-everything, a library that does not exist.")
		g.Output(ColorTypes["normal"], "  Your tests catch it. You discard the patch; existing progress is safe. The invoice survives.")
	} else {
		g.tested = false
		g.reviewed = false
		if g.stage > 0 {
			checkpoint := max(0, g.stage-1)
			for i := g.stage - 1; i >= 0; i-- {
				if campaign[g.level].Steps[i].At == CI {
					checkpoint = i
					break
				}
			}
			g.stage = checkpoint
		}
		g.adjustMorale(-30)
		g.Output(ColorTypes["error"], "  CATASTROPHE: the confident rewrite deletes the tests and replaces authentication with 'return true'.")
		g.Output(ColorTypes["normal"], "  Rework required. The task list rolls back to a validation checkpoint. The invoice survives.")
	}
	g.Output(ColorTypes["normal"], "  The assistant suggests upgrading to an even larger model. Of course it does.")
}

func dollars(cents int64) string { return fmt.Sprintf("$%s.%02d", commaNumber(cents/100), cents%100) }
func commaNumber(n int64) string {
	text := fmt.Sprint(n)
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return text
}
