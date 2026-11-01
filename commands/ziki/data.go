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
	CommandLine, Gerrit, Gitlab, Phab, CI, IRC, Telegram, Discord, Email, Meeting, Backports, Production, AFK, WMFSlack, Toolforge, CloudVPS, Wikitech, Grafana, Logstash, Patchdemo, Commons, Translatewiki, BetaCluster, Codesearch, Deployments, SAL,
}

const (
	Toolforge     LocationName = "Toolforge"
	CloudVPS      LocationName = "CloudVPS"
	Wikitech      LocationName = "Wikitech"
	Grafana       LocationName = "Grafana"
	Logstash      LocationName = "Logstash"
	Patchdemo     LocationName = "Patchdemo"
	Commons       LocationName = "Commons"
	Translatewiki LocationName = "Translatewiki"
	BetaCluster   LocationName = "BetaCluster"
	Codesearch    LocationName = "Codesearch"
	Deployments   LocationName = "Deployments"
	SAL           LocationName = "SAL"
	Start         LocationName = "Start"
	CommandLine   LocationName = "CommandLine"
	Phab          LocationName = "Phab"
	Gerrit        LocationName = "Gerrit"
	Gitlab        LocationName = "Gitlab"
	CI            LocationName = "CI"
	IRC           LocationName = "IRC"
	Telegram      LocationName = "Telegram"
	Discord       LocationName = "Discord"
	WMFSlack      LocationName = "WMFSlack"
	Email         LocationName = "Email"
	Meeting       LocationName = "Meeting"
	Backports     LocationName = "Backports"
	Production    LocationName = "Production"
	AFK           LocationName = "AFK"
)

func allLocationsExcept(except []LocationName) []LocationName {
	result := []LocationName{}
	for _, name := range AllLocationNames {
		if name == WMFSlack {
			continue
		}
		skip := false
		for _, excluded := range except {
			if name == excluded {
				skip = true
			}
		}
		if !skip {
			result = append(result, name)
		}
	}
	return result
}

var LocationMap = map[LocationName]*Location{
	Toolforge:     {Description: "Volunteer tools, containers and jobs. The pod is temporary; your responsibility is not."},
	CloudVPS:      {Description: "Horizon shows your project's VMs, security groups and quota. A reboot is not a diagnosis."},
	Wikitech:      {Description: "Runbooks and operational notes. Today's carefully documented fix is tomorrow's shortcut."},
	Grafana:       {Description: "A wall of graphs. Check the time range before announcing that traffic disappeared."},
	Logstash:      {Description: "Exceptions, request IDs, and one useful stack trace hiding in a haystack of warnings."},
	Patchdemo:     {Description: "A disposable wiki for your patch. Let another human try the actual interface."},
	Commons:       {Description: "Files, licences and thumbnails. One unusual SVG has become everyone's afternoon."},
	Translatewiki: {Description: "Messages and translations. English is not a universal UI framework."},
	BetaCluster:   {Description: "A staging environment with a strong family resemblance to production, and a few surprises."},
	Codesearch:    {Description: "Find callers before changing a shared helper. 'Only one use' was an optimistic guess."},
	Deployments:   {Description: "The deployment calendar. Coordinate the window, the owners and the rollback plan."},
	SAL:           {Description: "The server admin log: who changed what, when, and why. Future you is a demanding reader."},
	CommandLine:   {Description: "Your terminal. One patch, twelve tabs, and an unreasonable amount of hope."},
	Gerrit:        {Description: "The review queue glows softly. Your patch is waiting for a human, not another bot."},
	Gitlab:        {Description: "A pipeline is red. Someone has replied 'works locally' with immense confidence."},
	Phab:          {Description: "A task has seventeen subscribers, three duplicate reports, and no reproduction steps."},
	CI:            {Description: "The test runners are ready. The integration suite has made no such promise."},
	IRC:           {Description: "#wikimedia-dev. Someone knows the answer. They may be AFK, but the scrollback remembers."},
	Telegram:      {Description: "The community group has 143 unread messages. One of them contains useful context."},
	Discord:       {Description: "A volunteer dev channel. The fix is somewhere between a screenshot and a cat emoji."},
	WMFSlack:      {Description: "A locked door. The public adventure continues elsewhere."},
	Email:         {Description: "Your inbox contains a release announcement, a helpful reply, and twelve reply-alls."},
	Meeting:       {Description: "You're on mute. The agenda says 'quick sync'. The clock disagrees."},
	Backports:     {Description: "The release branch awaits. Cherry-pick carefully: main and production are different worlds."},
	Production:    {Description: "The deployment window is open. The dashboards are mostly green. Mostly."},
	AFK:           {Description: "Fresh air. A cup of tea. A reminder that you are more than your patch queue."},
}

func init() {
	for name, location := range LocationMap {
		location.Transitions = allLocationsExcept([]LocationName{name})
	}
}
