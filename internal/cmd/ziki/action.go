package ziki

import (
	"math/rand"
	"time"
)

type Action struct {
	base  int
	bonus int
	Name  string
}

func (a *Action) Use() int {
	return a.base + rand.Intn(a.bonus)
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
