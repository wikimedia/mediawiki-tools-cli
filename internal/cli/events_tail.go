package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TailEvents prints the last lines pending events and follows local queue files.
// Events submitted entirely between polls cannot be observed.
func (e *Events) TailEvents(ctx context.Context, output io.Writer, lines int) error {
	if lines < 0 {
		return fmt.Errorf("event line count must be zero or greater")
	}
	tailer := &eventTailer{events: e}
	defer tailer.close()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	initial := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		events, err := tailer.read()
		if err != nil {
			return err
		}
		if initial && len(events) > lines {
			events = events[len(events)-lines:]
		}
		initial = false
		for _, event := range events {
			if _, err := fmt.Fprintln(output, event); err != nil {
				return fmt.Errorf("could not write tailed event: %w", err)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

type tailedEventFile struct {
	file    *os.File
	info    os.FileInfo
	offset  int64
	partial string
	present bool
}

type eventTailer struct {
	events *Events
	files  []*tailedEventFile
}

func (w *eventTailer) close() {
	for _, file := range w.files {
		_ = file.file.Close()
	}
	w.files = nil
}

func (w *eventTailer) read() ([]string, error) {
	unlock, err := acquireEventsLock(filepath.Join(w.events.FallbackDir, "queue.lock"), time.Second)
	if err != nil {
		return nil, fmt.Errorf("could not lock telemetry queue for tailing: %w", err)
	}
	defer unlock()
	paths, err := w.paths()
	if err != nil {
		return nil, err
	}
	for _, file := range w.files {
		file.present = false
	}
	for _, path := range paths {
		if err := w.open(path); err != nil {
			return nil, fmt.Errorf("could not tail telemetry queue %s: %w", path, err)
		}
	}
	var events []string
	for _, file := range w.files {
		lines, err := file.read()
		if err != nil {
			return nil, fmt.Errorf("could not read tailed telemetry queue %s: %w", file.file.Name(), err)
		}
		events = append(events, lines...)
	}
	// Drain old descriptors before closing them: an uploader may have renamed
	// or deleted a queue after the last poll, with unread events still in it.
	kept := w.files[:0]
	for _, file := range w.files {
		if file.present {
			kept = append(kept, file)
		} else {
			_ = file.file.Close()
		}
	}
	w.files = kept
	return events, nil
}

func (w *eventTailer) paths() ([]string, error) {
	var paths []string
	for _, queue := range w.events.EventPaths() {
		dir := filepath.Dir(queue)
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not inspect telemetry directory %s: %w", dir, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), filepath.Base(queue)+".pending-") {
				paths = append(paths, filepath.Join(dir, entry.Name()))
			}
		}
	}
	return append(paths, w.events.EventPaths()...), nil
}

func (w *eventTailer) open(path string) error {
	file, err := os.Open(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return fmt.Errorf("expected a regular event file")
	}
	for _, existing := range w.files {
		if os.SameFile(existing.info, info) {
			existing.present = true
			_ = file.Close()
			return nil
		}
	}
	w.files = append(w.files, &tailedEventFile{file: file, info: info, present: true})
	return nil
}

func (f *tailedEventFile) read() ([]string, error) {
	info, err := f.file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < f.offset {
		if _, err := f.file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		f.offset, f.partial = 0, ""
	}
	reader := bufio.NewReader(f.file)
	var lines []string
	for {
		part, err := reader.ReadString('\n')
		f.offset += int64(len(part))
		f.partial += part
		if err == nil {
			if line := strings.TrimSpace(f.partial); line != "" {
				lines = append(lines, line)
			}
			f.partial = ""
		} else if errors.Is(err, io.EOF) {
			return lines, nil
		} else {
			return nil, err
		}
	}
}
