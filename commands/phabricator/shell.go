package phabricator

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
}

// runShell starts the interactive REPL.
func runShell(client *conduitClient, cfg *PhabConfig, startProject string, startTask string) error {
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
	}

	// Handle SIGINT as shell exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		<-sigCh
		fmt.Println("\nbye")
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		printPrompt(state)
		if !scanner.Scan() {
			// EOF (Ctrl-D)
			fmt.Println("\nbye")
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := handleShellLine(state, line); err != nil {
			if err == errExit {
				fmt.Println("bye")
				break
			}
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
	}

	signal.Stop(sigCh)
	return nil
}

var errExit = fmt.Errorf("exit")

func printPrompt(s *shellState) {
	cyan.Printf("%s", s.config.Username)
	fmt.Printf("@")
	if s.projectName == "" {
		white.Printf("(no-project)")
	} else {
		white.Printf("%s", s.projectName)
	}
	if s.currentColumn != "" {
		fmt.Printf("/")
		white.Printf("%s", s.currentColumn)
		if s.currentTask != "" {
			fmt.Printf("/")
			white.Printf("%s", s.currentTask)
		}
	}
	fmt.Printf(" \u276F ")
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

	case "rmtag", "rm":
		return shellRMTag(s, args)

	case "setprio", "sp", "prio":
		return shellSetPrio(s, args)

	case "setstatus", "st":
		return shellSetStatus(s, args)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\nCommands: ls, cd, view, comments, read, mv, rmtag, setprio, setstatus, exit\n", cmd)
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
		renderTaskList(tasks)
		return nil
	}

	if len(args) == 0 {
		if s.currentColumn == "" {
			// List columns
			renderColumnList(s.columns)
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
		renderTaskList(tasks)
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
	renderTaskList(tasks)
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
	renderTask(task, s.client)
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
	renderComments(task, txs, s.client)
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
	fmt.Printf("Moved %s to %s\n", taskRef, colName)
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
		fmt.Fprintln(os.Stderr, "Multiple projects matched:")
		for name, phid := range projects {
			fmt.Fprintf(os.Stderr, "  %s (%s)\n", name, phid)
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
	fmt.Printf("Removed tag from %s\n", taskRef)
	return nil
}

var priorityMap = map[string]string{
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
	fmt.Printf("%s priority set to: %s\n", taskRef, task.Fields.Priority.Name)
	return nil
}

var statusMap = map[string]string{
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
	fmt.Printf("%s status set to: %s\n", taskRef, task.Fields.Status.Name)
	return nil
}

// resolveTaskArg returns the task ref from args[0] if provided, otherwise the current task.
func resolveTaskArg(s *shellState, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return s.currentTask
}
