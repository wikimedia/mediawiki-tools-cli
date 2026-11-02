package phabricator

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	"golang.org/x/term"
)

// Bot PHIDs to filter from comments.
var botPHIDs = map[string]bool{
	"PHID-USER-idceizaw6elwiwm5xshb": true,
	"PHID-USER-gzxwjo5i7iwtojwoibwt": true,
	"PHID-USER-j4uyesgqhubl2dywl4xd": true,
}

var (
	yellow  = color.New(color.FgYellow, color.Bold)
	white   = color.New(color.FgWhite, color.Bold)
	blue    = color.New(color.FgBlue, color.Bold)
	magenta = color.New(color.FgMagenta, color.Bold)
	green   = color.New(color.FgGreen, color.Bold)

	yellowPlain = color.New(color.FgYellow)
	whitePlain  = color.New(color.FgWhite)
)

// termWidth returns the terminal width, defaulting to 80.
func termWidth() int {
	fd := int(os.Stdout.Fd())
	if w, _, err := term.GetSize(fd); err == nil && w > 0 {
		return w
	}
	return 80
}

// dashes returns a string of dashes across the terminal.
func dashes() string {
	return strings.Repeat("-", termWidth())
}

// normaliseLength pads or truncates s to length.
func normaliseLength(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}

// formatDate converts a Unix timestamp to a human-readable date string.
func formatDate(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04") + " UTC"
}

// renderMarkdown renders markdown text using glamour.
func renderMarkdown(md string) string {
	rendered, err := glamour.Render(md, "dark")
	if err != nil {
		return md
	}
	return rendered
}

// renderTask prints the full task view to the requested output.
func renderTask(w io.Writer, task *phabTask, client *conduitClient) {
	fmt.Fprintln(w)

	taskID := fmt.Sprintf("T%d", task.ID)
	taskURI := fmt.Sprintf("%s/T%d", client.baseURL, task.ID)
	yellow.Fprintf(w, "%s", taskID)
	fmt.Fprintf(w, " ")
	white.Fprintf(w, "%s", task.Fields.Name)
	fmt.Fprintf(w, " ")
	blue.Fprintf(w, "%s", taskURI)
	fmt.Fprintln(w)
	fmt.Fprintln(w)

	magenta.Fprintf(w, "Created: ")
	fmt.Fprintf(w, "%s", formatDate(task.Fields.DateCreated))
	fmt.Fprintf(w, "  ")
	magenta.Fprintf(w, "Last Modified: ")
	fmt.Fprintf(w, "%s\n", formatDate(task.Fields.DateModified))

	yellow.Fprintln(w, dashes())

	md := task.Fields.Description.Raw
	if md != "" {
		fmt.Fprint(w, renderMarkdown(md))
	}

	yellow.Fprintln(w, dashes())

	// Resolve author and assignee
	phids := []string{}
	if task.Fields.AuthorPHID != "" {
		phids = append(phids, task.Fields.AuthorPHID)
	}
	ownerPHID := ""
	if op, ok := task.Fields.OwnerPHID.(string); ok && op != "" {
		ownerPHID = op
		phids = append(phids, ownerPHID)
	}
	users, _ := client.getUsers(phids)

	authorName := task.Fields.AuthorPHID
	if u, ok := users[task.Fields.AuthorPHID]; ok {
		authorName = u[0]
	}
	assigneeName := "(none)"
	if ownerPHID != "" {
		if u, ok := users[ownerPHID]; ok {
			assigneeName = u[0]
		} else {
			assigneeName = ownerPHID
		}
	}

	blue.Fprintf(w, "Author: ")
	fmt.Fprintf(w, "%s", authorName)
	fmt.Fprintf(w, ", ")
	blue.Fprintf(w, "Assignee: ")
	fmt.Fprintf(w, "%s\n", assigneeName)

	magenta.Fprintf(w, "Status: ")
	fmt.Fprintf(w, "%s", task.Fields.Status.Name)
	fmt.Fprintf(w, "  ")
	magenta.Fprintf(w, "Priority: ")
	fmt.Fprintf(w, "%s\n", task.Fields.Priority.Name)

	// Projects
	if len(task.Attachments.Projects.ProjectPHIDs) > 0 {
		projNames, _ := client.getProjects(task.Attachments.Projects.ProjectPHIDs)
		names := make([]string, 0, len(projNames))
		for _, n := range projNames {
			names = append(names, n)
		}
		sort.Strings(names)
		green.Fprintf(w, "Projects: ")
		fmt.Fprintln(w, strings.Join(names, ", "))
	}

	fmt.Fprintln(w)
}

// renderComments prints the comments for a task to the requested output.
func renderComments(w io.Writer, task *phabTask, txs []phabTransaction, client *conduitClient) {
	// Collect author PHIDs (filtering bots)
	type commentEntry struct {
		authorPHID string
		date       int64
		content    string
	}

	var comments []commentEntry
	for _, tx := range txs {
		if tx.Type != "comment" {
			continue
		}
		if botPHIDs[tx.AuthorPHID] {
			continue
		}
		// Get latest non-removed version
		var latestVersion *struct {
			Version      int
			Removed      bool
			Content      string
			DateModified int64
		}
		for _, comment := range tx.Comments {
			if comment.Removed {
				continue
			}
			if latestVersion == nil || comment.Version > latestVersion.Version {
				v := struct {
					Version      int
					Removed      bool
					Content      string
					DateModified int64
				}{
					Version:      comment.Version,
					Removed:      comment.Removed,
					Content:      comment.Content.Raw,
					DateModified: comment.DateModified,
				}
				latestVersion = &v
			}
		}
		if latestVersion == nil {
			continue
		}
		comments = append(comments, commentEntry{
			authorPHID: tx.AuthorPHID,
			date:       tx.DateCreated,
			content:    latestVersion.Content,
		})
	}

	if len(comments) == 0 {
		fmt.Fprintln(w, "(no comments)")
		return
	}

	// Collect unique author PHIDs
	authorSet := make(map[string]bool)
	for _, c := range comments {
		authorSet[c.authorPHID] = true
	}
	phidList := make([]string, 0, len(authorSet))
	for p := range authorSet {
		phidList = append(phidList, p)
	}
	users, _ := client.getUsers(phidList)

	for _, c := range comments {
		authorName := c.authorPHID
		if u, ok := users[c.authorPHID]; ok {
			authorName = u[0]
		}
		fmt.Fprintln(w)
		yellow.Fprintf(w, "On %s, %s wrote:\n", formatDate(c.date), authorName)
		fmt.Fprint(w, renderMarkdown(c.content))
		whitePlain.Fprintln(w, strings.Repeat("-", termWidth()))
		fmt.Fprintln(w)
	}
}

// renderTaskList prints a table of tasks in a column.
func renderTaskListTo(w io.Writer, tasks []taskSummary) {
	if len(tasks) == 0 {
		fmt.Fprintln(w, "(no tasks)")
		return
	}
	for _, t := range tasks {
		id := normaliseLength(t.ID, 8)
		title := normaliseLength(t.Title, 60)
		yellowPlain.Fprint(w, id)
		fmt.Fprintf(w, "%s\n", title)
	}
}

// renderColumnList prints all column names.
func renderColumnListTo(w io.Writer, columns map[string]string) {
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		yellow.Fprintln(w, normaliseLength(name, 30))
	}
}
