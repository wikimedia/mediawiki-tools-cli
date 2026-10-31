package wiki

// Shared catalogue: MediaWiki 1.46.2 core, plus OAuth and Echo REL1_46 grants.
// Source mappings and their known gaps are documented in internal/wikiauth/README.md.
// This is never filtered by consumer ID; the authorization server validates availability.
const oauthCatalogueVersion = "MediaWiki 1.46.2 + OAuth/Echo REL1_46"

type oauthGrant struct{ Scope, Label, Group, Risk string }

var defaultOAuthGrants = []oauthGrant{
	{"basic", "Basic rights", "hidden", "low"},
	{"highvolume", "High-volume (bot) access", "high-volume", "low"},
	{"import", "Import revisions", "administration", "security"},
	{"editpage", "Edit existing pages", "page-interaction", "low"},
	{"editprotected", "Edit protected pages", "page-interaction", "vandalism"},
	{"editmycssjs", "Edit your user CSS/JSON/JavaScript", "customization", "security"},
	{"editmyoptions", "Edit your user preferences and JSON configuration", "customization", "security"},
	{"editinterface", "Edit the MediaWiki namespace and sitewide/user JSON", "administration", "vandalism"},
	{"editsiteconfig", "Edit sitewide and user CSS/JS", "administration", "security"},
	{"createeditmovepage", "Create, edit, and move pages", "page-interaction", "low"},
	{"uploadfile", "Upload new files", "file-interaction", "low"},
	{"uploadeditmovefile", "Upload, replace, and move files", "file-interaction", "low"},
	{"patrol", "Patrol changes to pages", "page-interaction", "low"},
	{"rollback", "Rollback changes to pages", "administration", "low"},
	{"blockusers", "Block and unblock users", "administration", "vandalism"},
	{"viewdeleted", "View deleted files and pages", "administration", "vandalism"},
	{"viewrestrictedlogs", "View restricted log entries", "administration", "security"},
	{"delete", "Delete pages, revisions, and log entries", "administration", "vandalism"},
	{"oversight", "Hide users and suppress revisions", "administration", "security"},
	{"protect", "Protect and unprotect pages", "administration", "vandalism"},
	{"viewmywatchlist", "View your watchlist", "watchlist-interaction", "low"},
	{"editmywatchlist", "Edit your watchlist", "other", "unknown"},
	{"sendemail", "Send email to other users", "email", "security"},
	{"createaccount", "Create accounts", "administration", "low"},
	{"privateinfo", "Access private information", "private-information", "low"},
	{"mergehistory", "Merge page histories", "administration", "vandalism"},
	{"oauthmanageownclient", "Manage your OAuth clients", "administration", "internal"},
	{"echo-read-notifications", "Read my notifications", "private-information", "low"},
}

var oauthGroupLabels = map[string]string{
	"high-volume":           "Perform high volume activity",
	"administration":        "Perform administrative actions",
	"page-interaction":      "Interact with pages",
	"customization":         "Customization and preferences",
	"file-interaction":      "Interact with media",
	"watchlist-interaction": "Interact with your watchlist",
	"other":                 "Miscellaneous activity",
	"email":                 "Send email",
	"private-information":   "Access private data about you",
	"unknown":               "Unknown grants",
}
