package dockercompose

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// Docker prints an ID on stdout as each container finishes starting. Retain the
// original output for partial-failure reporting while delivering live updates.
type containerStartOutput struct {
	output    bytes.Buffer
	pending   string
	completed func(string)
}

func (w *containerStartOutput) Write(data []byte) (int, error) {
	w.output.Write(data)
	w.pending += string(data)
	for {
		line, rest, ok := strings.Cut(w.pending, "\n")
		if !ok {
			break
		}
		w.completed(strings.TrimSpace(line))
		w.pending = rest
	}
	return len(data), nil
}

func (w *containerStartOutput) flush() {
	if w.pending != "" {
		w.completed(strings.TrimSpace(w.pending))
		w.pending = ""
	}
}

type containerStartRow struct {
	id, name string
	state    string
	finished time.Time
}

// containerStartProgress owns only its output rows. It never changes terminal
// input mode, uses an alternate screen, or intercepts process signals.
type containerStartProgress struct {
	mu            sync.Mutex
	writer        io.Writer
	rows          []containerStartRow
	started       time.Time
	width, height int
	drawn         int
	colour        bool
	failed        bool
	stop, done    chan struct{}
}

func newContainerStartProgress(stdout, stderr io.Writer, ids []string, names map[string]string) *containerStartProgress {
	out, stdoutOK := stdout.(*os.File)
	errout, stderrOK := stderr.(*os.File)
	if !stdoutOK || !stderrOK || !term.IsTerminal(int(out.Fd())) || !term.IsTerminal(int(errout.Fd())) || os.Getenv("TERM") == "dumb" {
		return nil
	}
	width, height, err := term.GetSize(int(errout.Fd()))
	if err != nil || width < 20 || height < 5 {
		return nil
	}
	return startContainerProgress(stderr, ids, names, width, height, os.Getenv("NO_COLOR") == "" && os.Getenv("CLICOLOR") != "0")
}

func startContainerProgress(writer io.Writer, ids []string, names map[string]string, width, height int, colour bool) *containerStartProgress {
	p := &containerStartProgress{writer: writer, started: time.Now(), width: width, height: height, colour: colour, stop: make(chan struct{}), done: make(chan struct{})}
	for _, id := range ids {
		name := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, ansi.Strip(names[id]))
		p.rows = append(p.rows, containerStartRow{id: id, name: name, state: "Starting"})
	}
	p.draw()
	go func() {
		defer close(p.done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-ticker.C:
				p.mu.Lock()
				p.draw()
				p.mu.Unlock()
			}
		}
	}()
	return p
}

func (p *containerStartProgress) complete(id string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.rows {
		if p.rows[i].id == id && p.rows[i].state == "Starting" {
			p.rows[i].state = "Started"
			p.rows[i].finished = time.Now()
		}
	}
	p.draw()
}

func (p *containerStartProgress) finish(err error) {
	if p == nil {
		return
	}
	close(p.stop)
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failed = err != nil
	for i := range p.rows {
		if p.rows[i].state == "Starting" {
			p.rows[i].state = "Started"
			if err != nil {
				p.rows[i].state = "Error"
			}
			p.rows[i].finished = time.Now()
		}
	}
	p.draw()
}

// draw is called while holding mu, except for the initial render before the
// ticker is started. Keep every row shorter than the terminal to avoid wrapping.
func (p *containerStartProgress) draw() {
	if file, ok := p.writer.(*os.File); ok {
		if width, height, err := term.GetSize(int(file.Fd())); err == nil && width >= 20 && height >= 5 {
			p.width, p.height = width, height
		}
	}
	now := time.Now()
	complete := 0
	for _, row := range p.rows {
		if row.state == "Started" {
			complete++
		}
	}
	mark := "+"
	if p.failed {
		mark = "!"
	}
	lines := []string{fmt.Sprintf("[%s] start %d/%d", mark, complete, len(p.rows))}
	visible := min(len(p.rows), max(1, p.height-3))
	nameWidth := 0
	for _, row := range p.rows[:visible] {
		nameWidth = max(nameWidth, ansi.StringWidth(row.name))
	}
	nameWidth = max(1, min(nameWidth, p.width-31))
	for _, row := range p.rows[:visible] {
		elapsed := now.Sub(p.started)
		if !row.finished.IsZero() {
			elapsed = row.finished.Sub(p.started)
		}
		icon, code := "⠋", "36"
		switch row.state {
		case "Starting":
			frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			icon = frames[int(elapsed.Milliseconds()/100)%len(frames)]
		case "Started":
			icon, code = "✔", "32"
		case "Error":
			icon, code = "✘", "31"
		}
		status := fmt.Sprintf("%-8s", row.state)
		if p.colour {
			icon = "\x1b[" + code + "m" + icon + "\x1b[0m"
			status = "\x1b[" + code + "m" + status + "\x1b[0m"
		}
		suffix := fmt.Sprintf(" %s %5.1fs", status, elapsed.Seconds())
		prefix := " " + icon + " Container "
		name := ansi.Truncate(row.name, nameWidth, "…")
		name += strings.Repeat(" ", max(0, nameWidth-ansi.StringWidth(name)))
		lines = append(lines, prefix+name+suffix)
	}
	if visible < len(p.rows) {
		lines = append(lines, fmt.Sprintf(" … %d more containers", len(p.rows)-visible))
	}
	if p.drawn > 0 {
		fmt.Fprintf(p.writer, "\x1b[%dA", p.drawn)
	}
	for _, line := range lines {
		fmt.Fprintf(p.writer, "\r\x1b[2K%s\n", ansi.Truncate(line, max(1, p.width-1), "…"))
	}
	if extra := p.drawn - len(lines); extra > 0 {
		for i := 0; i < extra; i++ {
			fmt.Fprint(p.writer, "\r\x1b[2K\n")
		}
		fmt.Fprintf(p.writer, "\x1b[%dA", extra)
	}
	p.drawn = len(lines)
}
