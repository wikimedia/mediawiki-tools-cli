package ziki

// A level is an ordered itinerary. Repeated destinations are intentional:
// they can have different tasks, or repeat a test after another change.
// Adding a level only requires a story and a list of {destination, task} pairs.
type missionStep struct {
	At   LocationName
	Task string
}

type level struct {
	Title string
	Role  string
	Brief string
	Steps []missionStep
}

var campaign = []level{
	{"Your first +2", "New contributor", "A small bug fix. Own your patch; the deployment team handles getting it live.", []missionStep{
		{CI, "tests"}, {Gerrit, "review"}, {Gerrit, "submit"},
	}},
	{"It works in every language", "Feature contributor", "A new interface needs a reproduction, a demo, translated messages and review. No production access yet.", []missionStep{
		{Phab, "reproduce"}, {Patchdemo, "demo"}, {Translatewiki, "messages"}, {CI, "tests"}, {Gerrit, "review"}, {Patchdemo, "demo"}, {CI, "retest"}, {Gerrit, "submit"},
	}},
	{"Your tool, your pager", "Tool maintainer", "A volunteer tool crashes after deploy. Work in its tool account and verify recovery; Wikimedia production is someone else's job.", []missionStep{
		{Toolforge, "toolLogs"}, {Gitlab, "tests"}, {Toolforge, "toolBuild"}, {Toolforge, "toolDeploy"}, {Grafana, "verify"}, {Wikitech, "document"},
	}},
	{"The impossible thumbnail", "Media feature maintainer", "A Commons file breaks thumbnail generation. Respect licensing, reproduce the bug and test the fix with real media.", []missionStep{
		{Commons, "media"}, {Phab, "reproduce"}, {Patchdemo, "demo"}, {Translatewiki, "messages"}, {CI, "tests"}, {Gerrit, "review"}, {Commons, "mediaVerify"}, {CI, "retest"}, {Gerrit, "submit"},
	}},
	{"The disk is full", "Cloud VPS project admin", "Your project's VM is unhealthy. Diagnose it, agree a safe cleanup and verify service health. No production hosts involved.", []missionStep{
		{CloudVPS, "vpsInspect"}, {CloudVPS, "diskLogs"}, {Wikitech, "runbook"}, {CloudVPS, "diskFix"}, {Grafana, "verify"}, {Wikitech, "document"},
	}},
	{"Backport duty", "Backport coordinator", "A reviewed fix is on main, but an older release branch needs a carefully scoped patch. Hand it to the deployer.", []missionStep{
		{Phab, "reproduce"}, {CI, "tests"}, {Gerrit, "review"}, {Gerrit, "submit"}, {Backports, "cherryPick"}, {CI, "branchTests"}, {Backports, "backportReview"}, {Deployments, "schedule"},
	}},
	{"Your first deployment", "Supervised deployer", "You are now responsible for a monitored rollout. Stage it, canary it, observe it, and leave a useful log.", []missionStep{
		{Wikitech, "runbook"}, {Deployments, "schedule"}, {BetaCluster, "staging"}, {Production, "canary"}, {Grafana, "verify"}, {Logstash, "verifyLogs"}, {Production, "rollout"}, {SAL, "log"},
	}},
	{"The robots are hungry", "Incident responder", "AI scrapers overload expensive pages. Follow the evidence and protect readers while containing the flood.", []missionStep{
		{Grafana, "scraperGraphs"}, {Logstash, "scraperLogs"}, {IRC, "coordinate"}, {Production, "rateLimit"}, {Grafana, "partial"}, {Logstash, "cacheClue"}, {Production, "cacheFix"}, {Grafana, "verify"}, {Wikitech, "document"},
	}},
	{"One helper, many callers", "Cross-component maintainer", "An innocent helper change affects several extensions. Inventory callers, test the interface and revisit review after fixing them.", []missionStep{
		{Phab, "reproduce"}, {Codesearch, "callers"}, {Patchdemo, "demo"}, {Gerrit, "review"}, {Codesearch, "callers"}, {CI, "retest"}, {Patchdemo, "demo"}, {Gerrit, "submit"},
	}},
	{"But beta was green", "Release validator", "Staging exposes a configuration-dependent bug. Compare evidence, fix the difference, and repeat validation before rollout.", []missionStep{
		{BetaCluster, "staging"}, {Logstash, "configClue"}, {Codesearch, "callers"}, {Gerrit, "review"}, {CI, "retest"}, {Gerrit, "submit"}, {BetaCluster, "staging"}, {Grafana, "verify"}, {Wikitech, "document"},
	}},
	{"All aboard the train", "Train conductor", "Coordinate the release train with service owners. Advance only after staging and canary checks; record each operational handoff.", []missionStep{
		{Deployments, "schedule"}, {IRC, "coordinate"}, {BetaCluster, "staging"}, {Production, "canary"}, {Grafana, "verify"}, {Logstash, "verifyLogs"}, {Production, "rollout"}, {Grafana, "verify"}, {SAL, "log"},
	}},
	{"Two outages, one coincidence", "Duty engineer", "A tool outage and a Cloud VPS failure look related. Check each service independently before blaming shared infrastructure.", []missionStep{
		{Toolforge, "toolLogs"}, {CloudVPS, "vpsInspect"}, {CloudVPS, "diskLogs"}, {CloudVPS, "diskFix"}, {Toolforge, "toolBuild"}, {Toolforge, "toolDeploy"}, {Grafana, "verify"}, {Wikitech, "document"}, {SAL, "log"},
	}},
	// Expert round: longer, explicitly ordered diagnostic/rollback/retest loops.
	{"Expert: the translated regression", "Senior feature maintainer", "The first demo missed a translated edge case. Return to the evidence and repeat review and testing before merging.", []missionStep{
		{Commons, "media"}, {Phab, "reproduce"}, {Patchdemo, "demo"}, {Translatewiki, "messages"}, {CI, "tests"}, {Gerrit, "review"}, {Patchdemo, "demoFailure"}, {Translatewiki, "messageFix"}, {Codesearch, "callers"}, {CI, "retest"}, {Gerrit, "review"}, {Patchdemo, "demo"}, {CI, "retest"}, {Gerrit, "submit"},
	}},
	{"Expert: the canary says no", "Deployment lead", "A release-branch fix passes tests but fails on the canary. Roll it back, repair the backport and repeat the release checks.", []missionStep{
		{Backports, "cherryPick"}, {CI, "branchTests"}, {Backports, "backportReview"}, {BetaCluster, "staging"}, {Production, "canary"}, {Grafana, "regression"}, {Logstash, "configClue"}, {Production, "rollback"}, {Backports, "backportFix"}, {CI, "branchTests"}, {Backports, "backportReview"}, {BetaCluster, "staging"}, {Production, "canary"}, {Grafana, "verify"}, {Logstash, "verifyLogs"}, {Production, "rollout"}, {SAL, "log"},
	}},
	{"Expert: stop the train", "Release incident coordinator", "The train uncovers an extension compatibility bug. Halt expansion, coordinate owners, test a fix and repeat the train checks.", []missionStep{
		{Deployments, "schedule"}, {BetaCluster, "staging"}, {Production, "canary"}, {Grafana, "regression"}, {Production, "rollback"}, {IRC, "coordinate"}, {Codesearch, "callers"}, {Phab, "reproduce"}, {Gerrit, "review"}, {CI, "retest"}, {Gerrit, "submit"}, {Backports, "cherryPick"}, {CI, "branchTests"}, {Backports, "backportReview"}, {BetaCluster, "staging"}, {Deployments, "schedule"}, {Production, "canary"}, {Grafana, "verify"}, {Production, "rollout"}, {SAL, "log"},
	}},
	{"Expert: the long night", "Incident commander", "Scrapers, a cache regression and a failing community tool overlap. Contain the incident, fix each cause, and verify twice before closing it.", []missionStep{
		{Grafana, "scraperGraphs"}, {Logstash, "scraperLogs"}, {IRC, "coordinate"}, {Production, "rateLimit"}, {Grafana, "partial"}, {Logstash, "cacheClue"}, {Codesearch, "callers"}, {Gerrit, "review"}, {CI, "retest"}, {Gerrit, "submit"}, {Backports, "cherryPick"}, {CI, "branchTests"}, {Backports, "backportReview"}, {Production, "canary"}, {Grafana, "regression"}, {Production, "rollback"}, {Backports, "backportFix"}, {CI, "branchTests"}, {Backports, "backportReview"}, {Production, "canary"}, {Grafana, "verify"}, {Production, "rollout"}, {Toolforge, "toolLogs"}, {Toolforge, "toolDeploy"}, {Grafana, "verify"}, {SAL, "log"}, {Wikitech, "document"},
	}},
}

// Each task has one sound decision and one tempting mistake. The engine
// alternates their order, so "always press 1" is not a winning strategy.
type missionTask struct {
	Title  string
	Good   string
	Bad    string
	Result string
}

var missionTasks = map[string]missionTask{
	"tests":          {"CI is red. What now?", "Reproduce the failure and fix the cause", "Retry until the runner feels appreciated", "Green tests, with a regression test that fails before the fix."},
	"retest":         {"The patch changed after review.", "Run the relevant tests again on this patchset", "Reuse the green badge from the previous patchset", "The revised patchset passes. Old evidence has been replaced with current evidence."},
	"review":         {"A reviewer asks about the edge cases.", "Address the feedback and explain the tradeoff", "Reply 'done' without changing anything", "Review feedback addressed. Validation must still match the final patch."},
	"submit":         {"The patch is reviewed and its required checks are complete.", "Submit the reviewed patch and hand off deployment", "Sneak in one last unreviewed change", "MERGED. Deployment belongs to the people responsible for this release."},
	"reproduce":      {"The report says 'broken'. Useful.", "Record steps, expected behaviour and a minimal example", "Close it as works-for-me", "A reproducible task gives everyone the same problem to solve."},
	"demo":           {"Someone needs to try the interface.", "Build a Patchdemo and ask someone else to test it", "Treat a screenshot as equivalent to testing", "A fresh demo confirms the user-facing behaviour."},
	"messages":       {"Your new label is hardcoded English.", "Use message keys and provide translator context", "Translate the string manually in the PHP file", "Messages are translatable and their context is documented."},
	"toolLogs":       {"A Toolforge tool is unavailable.", "Inspect that tool's job logs and quota", "Assume you can administer every tool as yourself", "The tool account's logs isolate the failed job. This is not a production server."},
	"toolBuild":      {"The tool needs a new container image.", "Build a pinned revision and check the build output", "Restart an old image and call that a build", "A reproducible build is ready for the tool's webservice."},
	"toolDeploy":     {"Deploy the volunteer tool.", "Use its tool account, configure the service and check health", "Store required state only in the container filesystem", "The tool is serving again; its state survives replacement of the container."},
	"verify":         {"How do you know it is healthy?", "Compare latency, errors and traffic with the baseline", "Look only at a green CPU graph", "Reader-facing latency, errors and traffic are healthy for the observation window."},
	"document":       {"The fix worked. What remains?", "Document the cause, recovery and follow-up work", "Leave 'fixed it' with no explanation", "The runbook now helps the next person rather than preserving a mystery."},
	"media":          {"A Commons file triggers a failure.", "Check the file's licence and reproduce with a minimal sample", "Reupload a random copyrighted file as a test", "A suitable media sample reproduces the thumbnail bug."},
	"mediaVerify":    {"Does the media fix really work?", "Check the original sample and another supported format", "Check only that the upload button still exists", "The affected media and a control format both render correctly."},
	"vpsInspect":     {"A Cloud VPS instance is unhealthy.", "Check its console, disk and project security groups", "Reboot every instance in the project", "The instance is reachable but its disk is full. Scope stays within your project."},
	"diskLogs":       {"The logs stop during writes.", "Correlate write failures with disk usage", "Delete all logs before reading any of them", "A runaway log is filling the volume; preserve evidence before cleanup."},
	"runbook":        {"Before changing anything operational...", "Read the runbook, check scope and prepare a rollback", "Assume your memory is the current procedure", "You have the relevant procedure, owners and rollback plan."},
	"diskFix":        {"Reclaim space safely.", "Preserve evidence, rotate the runaway log and fix retention", "Delete database files until the graph looks better", "Writes resume and retention prevents the same failure recurring."},
	"cherryPick":     {"The release branch differs from main.", "Cherry-pick only the fix and resolve conflicts deliberately", "Copy all of main into the release branch", "A minimal release-branch patch is ready for branch tests and review."},
	"branchTests":    {"The backport needs its own evidence.", "Test the actual release-branch patch and dependencies", "Point to main's CI result", "The release-branch checks pass for this backport."},
	"backportReview": {"Ready to approve the backport?", "Review the branch diff and its current test results", "Approve because the original author seems nice", "The scoped backport is reviewed and ready for a deployment window."},
	"schedule":       {"Choose a release window.", "Coordinate on the deployment calendar with owners and rollback cover", "Deploy during someone else's incident without telling them", "The window, owners and rollback coverage are agreed."},
	"staging":        {"Try the candidate in Beta Cluster.", "Exercise the affected path and note configuration differences", "Assume beta and production are identical", "Staging validation is recorded, including the limits of that evidence."},
	"canary":         {"Begin the rollout.", "Announce the change and deploy to a limited canary", "Deploy everywhere and immediately go for lunch", "The canary is running. Observe it before widening the rollout."},
	"verifyLogs":     {"Graphs alone do not tell the whole story.", "Check relevant exceptions, request IDs and the deployment time", "Count all historical warnings as new failures", "No new error pattern appears in the affected requests."},
	"rollout":        {"The canary checks passed.", "Expand gradually with monitoring and rollback readiness", "Disable alerts so the rollout stays green", "SHIPPED. The rollout completes with healthy service checks."},
	"log":            {"Leave an operational record.", "Record the change, time, reason and task in SAL", "Write 'stuff happened' tomorrow", "The server admin log records what changed and why."},
	"scraperGraphs":  {"Latency climbs while request volume explodes.", "Identify affected endpoints and compare uncached request rates", "Treat every request as a new human reader", "Expensive uncached requests correlate with the slowdown."},
	"scraperLogs":    {"What is generating that traffic?", "Inspect request patterns and expensive URLs", "Trust any User-Agent containing 'friendly'", "Repeated automated requests concentrate on expensive paths."},
	"coordinate":     {"Others need the same picture.", "Share evidence, scope and the current plan in IRC", "Run competing fixes privately", "Owners agree a scoped plan and know who is doing what."},
	"rateLimit":      {"AI scrapers hammer expensive pages.", "Apply targeted limits while protecting ordinary readers", "Block every reader: perfect latency!", "The request flood reduces. Check whether that was sufficient."},
	"partial":        {"Traffic fell, but latency remains high.", "Treat this as partial recovery and investigate further", "Declare victory because one graph improved", "The first mitigation helped, but the incident remains open."},
	"cacheClue":      {"Requests still bypass the cache.", "Trace the changed cache behaviour and identify the failing path", "Add unlimited capacity without investigating", "A cache regression explains the remaining expensive requests."},
	"cacheFix":       {"Contain the remaining expensive requests.", "Apply the scoped cache fix and validate reader behaviour", "Cache private responses publicly for speed", "Cache behaviour is repaired without exposing private responses."},
	"callers":        {"A shared helper is about to change.", "Find callers and check the compatibility contract", "Rename it and assume search-and-replace is enough", "Affected callers and their assumptions are identified."},
	"configClue":     {"A failure appears only with release configuration.", "Compare the failing request and configuration with staging", "Dismiss it because beta was green", "A configuration-dependent incompatibility is isolated."},
	"demoFailure":    {"The translated demo breaks the layout.", "Capture the locale and reproduction, then reopen the fix", "Tell users to switch to English", "The translated edge case is now part of the required fix."},
	"messageFix":     {"A translation needs different word order.", "Use complete messages with named parameters and context", "Concatenate English fragments more carefully", "The message supports translated word order without changing its meaning."},
	"regression":     {"Canary errors rise after the change.", "Stop expansion and prepare the known rollback", "Roll out faster to make the sample bigger", "The rollout is paused. Recovery takes priority over defending the patch."},
	"rollback":       {"Restore service before repairing the patch.", "Roll back the scoped change and verify the previous version", "Patch production by hand without recording it", "The previous version is restored; the faulty patch needs repair and retesting."},
	"backportFix":    {"Repair the release-branch incompatibility.", "Update the scoped patch and repeat tests and review", "Reuse the approval from before the rollback", "The revised backport is ready for fresh branch tests and review."},
}
