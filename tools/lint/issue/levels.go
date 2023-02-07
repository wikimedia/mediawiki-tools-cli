package issue

type Level uint32

const (
	ErrorLevel Level = iota
	WarningLevel
	SuggestLevel
)
