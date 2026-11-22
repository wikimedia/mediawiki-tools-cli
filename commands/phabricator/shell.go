package phabricator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
)

// shellState holds the current interactive shell state.
type shellState struct {
	client        *conduitClient
	config        *PhabConfig
	projectPHID   string
	projectName   string
	columns       map[string]string // normalized name -> PHID
	currentColumn string            // normalized column name
	currentTask   string            // task number like "T12345"
	in            io.Reader
	out           io.Writer
	errOut        io.Writer
}

// runShell starts the interactive REPL.
func runShell(ctx context.Context, in io.Reader, out, errOut io.Writer, client *conduitClient, cfg *PhabConfig, startProject string, startTask string) error {
	projectName := strings.TrimSpace(startProject)
	if projectName == "" {
		projectName = strings.TrimSpace(cfg.DefaultProject)
	}

	projectPHID := ""
	columns := map[string]string{}
	if projectName != "" {
		var err error
		projectPHID, err = client.lookupProjectPHID(projectName)
		if err != nil {
			return fmt.Errorf("looking up project %q: %w", projectName, err)
		}

		columns, err = client.getColumns(projectPHID)
		if err != nil {
			return fmt.Errorf("fetching columns: %w", err)
		}
	}

	state := &shellState{
		client:      client,
		config:      cfg,
		projectPHID: projectPHID,
		projectName: projectName,
		columns:     columns,
		currentTask: strings.ToUpper(strings.TrimSpace(startTask)),
		in:          in,
		out:         out,
		errOut:      errOut,
	}

	// Let the command return on Ctrl+C even while waiting for input.
	shellCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	scanner := bufio.NewScanner(in)
	for {
		if err := printPrompt(state); err != nil {
			return err
		}
		// Read only the requested line so an explicit exit cannot leave a
		// goroutine waiting for the next line of terminal input.
		scanned := make(chan bool, 1)
		go func() { scanned <- scanner.Scan() }()
		var hasLine bool
		select {
		case <-shellCtx.Done():
			// Terminal input and pipes can be closed to unblock the pending read.
			if closer, ok := in.(io.Closer); ok {
				_ = closer.Close()
			}
			if _, err := fmt.Fprintln(out, "\nbye"); err != nil {
				return err
			}
			return ctx.Err()
		case hasLine = <-scanned:
		}
		if !hasLine {
			// EOF (Ctrl-D)
			if _, err := fmt.Fprintln(out, "\nbye"); err != nil {
				return err
			}
			if err := scanner.Err(); err != nil {
				return err
			}
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := handleShellLine(state, line); err != nil {
			if err == errExit {
				if _, err := fmt.Fprintln(out, "bye"); err != nil {
					return err
				}
				break
			}
			if _, writeErr := fmt.Fprintln(errOut, "Error:", err); writeErr != nil {
				return writeErr
			}
		}
	}

	return nil
}

var errExit = fmt.Errorf("exit")

func printPrompt(s *shellState) error {
	if _, err := fmt.Fprintf(s.out, "%s@", s.config.Username); err != nil {
		return err
	}
	if s.projectName == "" {
		if _, err := fmt.Fprint(s.out, "(no-project)"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprint(s.out, s.projectName); err != nil {
			return err
		}
	}
	if s.currentColumn != "" {
		if _, err := fmt.Fprintf(s.out, "/%s", s.currentColumn); err != nil {
			return err
		}
		if s.currentTask != "" {
			if _, err := fmt.Fprintf(s.out, "/%s", s.currentTask); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprint(s.out, " \u276F ")
	return err
}

func requireProjectSelected(s *shellState, action string) error {
	if s.projectPHID == "" {
		return fmt.Errorf("no project selected; use --project to start shell in a project before using %s", action)
	}
	return nil
}

func handleShellLine(s *shellState, line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "exit", "quit":
		return errExit

	case "ls":
		return shellLS(s, args)

	case "cd":
		return shellCD(s, args)

	case "view", "v":
		return shellView(s, args)

	case "comments", "c":
		return shellComments(s, args)

	case "read", "r":
		// view + comments combined
		if err := shellView(s, args); err != nil {
			return err
		}
		return shellComments(s, args)

	case "mv":
		return shellMV(s, args)

	case "assign", "a":
		return shellAssign(s, args)

	case "rmtag", "rm":
		return shellRMTag(s, args)

	case "setprio", "sp", "prio":
		return shellSetPrio(s, args)

	case "setstatus", "st":
		return shellSetStatus(s, args)

	default:
		_, _ = fmt.Fprintf(s.errOut, "unknown command: %s\nCommands: ls, cd, view, comments, read, mv, assign, rmtag, setprio, setstatus, exit\n", cmd)
	}
	return nil
}

// shellLS — list columns or tasks in current/named column.
func shellLS(s *shellState, args []string) error {
	if s.projectPHID == "" {
		if len(args) > 0 {
			return fmt.Errorf("no project selected; use cd <project> first")
		}
		tasks, err := s.client.listTasksGlobal(100)
		if err != nil {
			return err
		}
		renderTaskListTo(s.out, tasks)
		return nil
	}

	if len(args) == 0 {
		if s.currentColumn == "" {
			// List columns
			renderColumnListTo(s.out, s.columns)
			return nil
		}
		// List tasks in current column
		colPHID, ok := s.columns[s.currentColumn]
		if !ok {
			return fmt.Errorf("column not found: %s", s.currentColumn)
		}
		tasks, err := s.client.listTasksInColumn(s.projectPHID, colPHID, 0)
		if err != nil {
			return err
		}
		renderTaskListTo(s.out, tasks)
		return nil
	}

	// ls <column>
	colName := normaliseColumnKey(args[0])
	colPHID, ok := s.columns[colName]
	if !ok {
		return fmt.Errorf("column not found: %s", args[0])
	}
	tasks, err := s.client.listTasksInColumn(s.projectPHID, colPHID, 0)
	if err != nil {
		return err
	}
	renderTaskListTo(s.out, tasks)
	return nil
}

// shellCD — change directory to a column, a task, or go up.
func shellCD(s *shellState, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cd <column> | cd <T12345> | cd ..")
	}
	target := args[0]

	if target == ".." {
		if s.currentTask != "" {
			s.currentTask = ""
		} else if s.currentColumn != "" {
			s.currentColumn = ""
		} else if s.projectPHID != "" {
			s.projectPHID = ""
			s.projectName = ""
			s.columns = map[string]string{}
		}
		return nil
	}

	// If it looks like a task number
	if isTaskRef(target) {
		s.currentTask = strings.ToUpper(target)
		return nil
	}

	// If no project yet, treat target as project name.
	if s.projectPHID == "" {
		projectPHID, err := s.client.lookupProjectPHID(target)
		if err != nil {
			return fmt.Errorf("project not found: %s", target)
		}
		columns, err := s.client.getColumns(projectPHID)
		if err != nil {
			return err
		}
		projectName := strings.TrimPrefix(strings.TrimSpace(target), "#")
		if projects, err := s.client.getProjects([]string{projectPHID}); err == nil {
			if name, ok := projects[projectPHID]; ok && strings.TrimSpace(name) != "" {
				projectName = name
			}
		}

		s.projectPHID = projectPHID
		s.projectName = projectName
		s.columns = columns
		s.currentColumn = ""
		s.currentTask = ""
		return nil
	}

	// Otherwise treat as column name in current project.
	colName := normaliseColumnKey(target)
	if _, ok := s.columns[colName]; !ok {
		return fmt.Errorf("column not found: %s", target)
	}
	s.currentColumn = colName
	s.currentTask = ""
	return nil
}

func isTaskRef(s string) bool {
	s = strings.ToUpper(s)
	if !strings.HasPrefix(s, "T") {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 1
}

// shellView — view a task.
func shellView(s *shellState, args []string) error {
	taskRef := resolveTaskArg(s, args)
	if taskRef == "" {
		return fmt.Errorf("no task selected; use: view <T12345>")
	}
	task, err := s.client.getTask(taskRef)
	if err != nil {
		return err
	}
	renderTask(s.out, task, s.client)
	return nil
}

// shellComments — show comments for a task.
func shellComments(s *shellState, args []string) error {
	taskRef := resolveTaskArg(s, args)
	if taskRef == "" {
		return fmt.Errorf("no task selected; use: comments <T12345>")
	}
	task, err := s.client.getTask(taskRef)
	if err != nil {
		return err
	}
	txs, err := s.client.getTransactions(task.PHID)
	if err != nil {
		return err
	}
	renderComments(s.out, task, txs, s.client)
	return nil
}

// shellMV — move task to another column.
func shellMV(s *shellState, args []string) error {
	if err := requireProjectSelected(s, "mv"); err != nil {
		return err
	}

	if len(args) < 1 {
		return fmt.Errorf("usage: mv <column> [task]")
	}
	colName := normaliseColumnKey(args[0])
	colPHID, ok := s.columns[colName]
	if !ok {
		return fmt.Errorf("column not found: %s", args[0])
	}
	taskRef := s.currentTask
	if len(args) >= 2 {
		taskRef = args[1]
	}
	if taskRef == "" {
		return fmt.Errorf("no task selected")
	}

	taskPHID, err := s.client.lookupTaskPHID(taskRef)
	if err != nil {
		return err
	}
	if err := s.client.moveTaskToColumn(taskPHID, colPHID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.out, "Moved %s to %s\n", taskRef, colName); err != nil {
		return err
	}
	return nil
}

// shellAssign assigns the current or named task to a Phabricator username.
func shellAssign(s *shellState, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: assign <username|me> [task]")
	}
	user, err := s.client.resolveUser(args[0])
	if err != nil {
		return err
	}
	taskRef := resolveTaskArg(s, args[1:])
	if taskRef == "" {
		return fmt.Errorf("no task selected")
	}
	taskPHID, err := s.client.lookupTaskPHID(taskRef)
	if err != nil {
		return err
	}
	if err := s.client.assignTask(taskPHID, user.PHID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.out, "Assigned %s to %s\n", taskRef, user.Fields.Username); err != nil {
		return err
	}
	return nil
}

// shellRMTag — remove a project tag from a task.
func shellRMTag(s *shellState, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rmtag <project> [task]")
	}
	tagQuery := args[0]
	taskRef := s.currentTask
	if len(args) >= 2 {
		taskRef = args[1]
	}
	if taskRef == "" {
		return fmt.Errorf("no task selected")
	}

	projects, err := s.client.findProjectsByName(tagQuery)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return fmt.Errorf("project %q not found", tagQuery)
	}
	if len(projects) > 1 {
		fmt.Fprintln(s.errOut, "Multiple projects matched:")
		for name, phid := range projects {
			fmt.Fprintf(s.errOut, "  %s (%s)\n", name, phid)
		}
		return fmt.Errorf("specify a more precise project name")
	}
	var projectPHID string
	for _, phid := range projects {
		projectPHID = phid
	}

	taskPHID, err := s.client.lookupTaskPHID(taskRef)
	if err != nil {
		return err
	}
	if err := s.client.removeProjectFromTask(taskPHID, projectPHID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.out, "Removed tag from %s\n", taskRef); err != nil {
		return err
	}
	return nil
}

var priorityMap = map[string]string{
	"unbreak": "unbreak", "triage": "triage", "high": "high", "normal": "normal", "low": "low", "lowest": "lowest",
	"u":  "unbreak",
	"t":  "triage",
	"h":  "high",
	"n":  "normal",
	"l":  "low",
	"ll": "lowest",
}

// shellSetPrio — set task priority.
func shellSetPrio(s *shellState, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: setprio <u|t|h|n|l|ll> [task]\n  u=unbreak t=triage h=high n=normal l=low ll=lowest")
	}
	prioKey := strings.ToLower(args[0])
	prioVal, ok := priorityMap[prioKey]
	if !ok {
		return fmt.Errorf("unknown priority %q; valid: u=unbreak t=triage h=high n=normal l=low ll=lowest", prioKey)
	}
	taskRef := s.currentTask
	if len(args) >= 2 {
		taskRef = args[1]
	}
	if taskRef == "" {
		return fmt.Errorf("no task selected")
	}

	taskPHID, err := s.client.lookupTaskPHID(taskRef)
	if err != nil {
		return err
	}
	if err := s.client.setTaskPriority(taskPHID, prioVal); err != nil {
		return err
	}
	// Show updated task
	task, err := s.client.getTaskByPHID(taskPHID)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.out, "%s priority set to: %s\n", taskRef, task.Fields.Priority.Name); err != nil {
		return err
	}
	return nil
}

var statusMap = map[string]string{
	"open": "open", "resolved": "resolved", "progress": "progress", "stalled": "stalled", "declined": "declined", "duplicate": "duplicate", "invalid": "invalid",
	"o":   "open",
	"r":   "resolved",
	"p":   "progress",
	"s":   "stalled",
	"d":   "declined",
	"dup": "duplicate",
	"i":   "invalid",
}

// shellSetStatus — set task status.
func shellSetStatus(s *shellState, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: setstatus <o|r|p|s|d|dup|i> [task]\n  o=open r=resolved p=progress s=stalled d=declined dup=duplicate i=invalid")
	}
	statusKey := strings.ToLower(args[0])
	statusVal, ok := statusMap[statusKey]
	if !ok {
		return fmt.Errorf("unknown status %q; valid: o=open r=resolved p=progress s=stalled d=declined dup=duplicate i=invalid", statusKey)
	}
	taskRef := s.currentTask
	if len(args) >= 2 {
		taskRef = args[1]
	}
	if taskRef == "" {
		return fmt.Errorf("no task selected")
	}

	taskPHID, err := s.client.lookupTaskPHID(taskRef)
	if err != nil {
		return err
	}
	if err := s.client.setTaskStatus(taskPHID, statusVal); err != nil {
		return err
	}
	// Show updated task
	task, err := s.client.getTaskByPHID(taskPHID)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.out, "%s status set to: %s\n", taskRef, task.Fields.Status.Name); err != nil {
		return err
	}
	return nil
}

// resolveTaskArg returns the task ref from args[0] if provided, otherwise the current task.
func resolveTaskArg(s *shellState, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return s.currentTask
}
