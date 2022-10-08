package ziki

var Messages = map[string]string{
	"welcome": "Welcome to Ziki: The Search for Merge!\nYou are a new developer trying to get your first change merged\nThe game ends when you either get your patch merged, or run out of Morale and abandon it.\nType 'help' for help, or 'quit' to exit.\n",
}

var ColorTypes = map[string]string{
	"normal":    "white",
	"error":     "red",
	"prompt":    "green",
	"alert":     "blue",
	"separator": "yellow",
}

var Actions = map[int]*Action{
	1: {Name: "Good suggestion", base: 1, bonus: 15},
	2: {Name: "Great suggestion", base: 10, bonus: 15},
	3: {Name: "Bad suggestion", base: -15, bonus: 15},
	4: {Name: "Terrible suggestion", base: -25, bonus: 15},
	5: {Name: "Thoughtful comment", base: 3, bonus: 12},
	6: {Name: "Snarky comment", base: -14, bonus: 12},
}

var Reviewers = map[int]*Actor{
	0: {Name: "Helpful Novice", Morale: 50, Actions: []int{1, 3, 4, 5}, Npc: true},
	1: {Name: "Helpful Peer", Morale: 55, Actions: []int{1, 2, 3, 5}, Npc: true},
	2: {Name: "Helpful Mentor", Morale: 55, Actions: []int{1, 2, 5}, Npc: true},
	3: {Name: "Hurtful Novice", Morale: 50, Actions: []int{1, 3, 4, 6}, Npc: true},
	4: {Name: "Hurtful Peer", Morale: 55, Actions: []int{1, 3, 4, 6}, Npc: true},
	5: {Name: "Hurtful Mentor", Morale: 55, Actions: []int{1, 3, 4, 6}, Npc: true},
}

var Events = map[string]*Event{
	"codeReview":   {Type: "CodeReview", Chance: 25, Description: "You receive a Code Review comment", Morale: 0, Evt: ""},
	"criticism":    {Type: "Story", Chance: 25, Description: "Someone criticises you unfairly", Morale: -40, Evt: ""},
	"wikilove":     {Type: "Story", Chance: 25, Description: "A colleague sends you wikilove", Morale: +40, Evt: ""},
	"unbreakNow":   {Type: "Story", Chance: 25, Description: "You broke the wikis", Morale: -30, Evt: ""},
	"extraHoliday": {Type: "Story", Chance: 20, Description: "You are granted an extra holiday", Morale: +30, Evt: "recharging"},
	"recharging":   {Type: "Story", Chance: 100, Description: "You do non-computer things you enjoy", Morale: +20, Evt: ""},
	"spam":         {Type: "Story", Chance: 75, Description: "You receive annoying spam", Morale: -5, Evt: ""},
}

type LocationName string

var AllLocationNames = []LocationName{
	CommandLine,
	Phab,
	Gerrit,
	Gitlab,
	Email,
	Chat,
	Meeting,
	AFK,
}

const (
	Start       LocationName = "Start"
	CommandLine LocationName = "CommandLine"
	Phab        LocationName = "Phab"
	Gerrit      LocationName = "Gerrit"
	Gitlab      LocationName = "Gitlab"
	Email       LocationName = "Email"
	Chat        LocationName = "Chat"
	Meeting     LocationName = "Meeting"
	AFK         LocationName = "AFK"
)

func allLocationsExcept(except []LocationName) []LocationName {
	allExcept := []LocationName{}
	for _, name := range AllLocationNames {
		skip := false
		for _, skipName := range except {
			if name == skipName {
				skip = true
			}
		}
		if !skip {
			allExcept = append(allExcept, name)
		}
	}
	return allExcept
}

var LocationMap = map[LocationName]*Location{
	CommandLine: {Description: "You just pushed your first change to Gerrit.", Transitions: allLocationsExcept([]LocationName{CommandLine}), Events: []string{}},
	Phab:        {Description: "You are looking at your Phabricator task.", Transitions: allLocationsExcept([]LocationName{Phab}), Events: []string{"codeReview", "criticism", "wikilove", "unbreakNow"}},
	Gerrit:      {Description: "You are looking at your change in Gerrit.", Transitions: allLocationsExcept([]LocationName{Gerrit}), Events: []string{"codeReview", "criticism", "wikilove", "unbreakNow"}},
	Gitlab:      {Description: "You are in Gitlab. There aren't many people here.", Transitions: allLocationsExcept([]LocationName{Gitlab}), Events: []string{"codeReview", "criticism", "wikilove", "unbreakNow"}},
	Email:       {Description: "You are in your email, looking at your massive inbox.", Transitions: allLocationsExcept([]LocationName{Email}), Events: []string{"codeReview", "criticism", "wikilove", "unbreakNow", "extraHoliday", "spam"}},
	Chat:        {Description: "You are in Slack/Element/IRC/whatever-you-prefer.", Transitions: allLocationsExcept([]LocationName{Chat}), Events: []string{"codeReview", "criticism", "wikilove", "unbreakNow"}},
	Meeting:     {Description: "You are in Google Meet", Transitions: AllLocationNames, Events: []string{"codeReview", "criticism", "unbreakNow", "wikilove", "extraHoliday"}},
	AFK:         {Description: "You are away from your computer, living your real life", Transitions: allLocationsExcept([]LocationName{AFK}), Events: []string{"recharging"}},
}
