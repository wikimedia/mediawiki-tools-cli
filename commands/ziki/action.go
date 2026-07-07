package ziki

import (
	"math/rand"
)

type Action struct {
	base  int
	bonus int
	Name  string
}

func (a *Action) Use() int {
	return a.base + rand.Intn(a.bonus) // #nosec G404
}
