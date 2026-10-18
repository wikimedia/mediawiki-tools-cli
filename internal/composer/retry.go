// Package composer provides helpers for automated dependency installation.
package composer

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

const outputLimit = 64 * 1024

var (
	ansiEscape       = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	curlFailure      = regexp.MustCompile(`(?i)\bcurl error\s+([0-9]+)\s+while downloading\b`)
	exceptionHeader  = regexp.MustCompile(`(?m)^[\t ]*In [^\r\n]+ line [0-9]+:[\t ]*\r?$`)
	permanentFailure = regexp.MustCompile(`(?i)` + strings.Join([]string{
		`your requirements could not be resolved`,
		`could not find a matching version`,
		`does not contain valid json|json validation error|does not match the expected json schema`,
		`is missing from your system`,
		`permission denied|not writable|failed to extract|could not delete`,
		`authentication required|invalid credentials`,
		`HTTP/[0-9.]+\s+(401|403)\b|\b(401 unauthorized|403 forbidden)\b`,
		`\bscript\b[\s\S]*\breturned\s+with\s+error\s+code\b`,
	}, "|"))
)

// RunWithRetry runs an automated Composer dependency command up to three times.
// Only explicit DNS, connection and timeout download failures qualify for retries.
// Output stays live, and the final command's exit code is returned unchanged.
func RunWithRetry(run func(stdout, stderr io.Writer) int, stdout, stderr io.Writer) int {
	delays := []time.Duration{2 * time.Second, 5 * time.Second}
	for attempt := 0; ; attempt++ {
		output := &attemptOutput{}
		code := run(io.MultiWriter(stdout, output), io.MultiWriter(stderr, output))
		// Only generic or transport failure statuses can qualify. In particular,
		// dependency resolution errors (2) and interruptions must not be retried.
		if code != 1 && code != 100 {
			return code
		}
		reason := output.networkFailure()
		if reason == "" {
			return code
		}
		if attempt == len(delays) {
			fmt.Fprintf(stderr, "Composer %s persisted after %d attempts. Check DNS and network access from the MediaWiki container; working host connectivity does not confirm container connectivity.\n", reason, attempt+1)
			return code
		}
		delay := delays[attempt]
		fmt.Fprintf(stderr, "Composer %s; retrying in %s (attempt %d/%d).\n", reason, delay, attempt+2, len(delays)+1)
		time.Sleep(delay)
	}
}

// Keep only a bounded tail of each attempt; Composer output can be very large.
// Stdout and stderr may arrive concurrently from separate process pipes.
type attemptOutput struct {
	mu   sync.Mutex
	tail []byte
}

func (o *attemptOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(p)
	if n >= outputLimit {
		o.tail = append(o.tail[:0], p[n-outputLimit:]...)
	} else {
		if overflow := len(o.tail) + n - outputLimit; overflow > 0 {
			copy(o.tail, o.tail[overflow:])
			o.tail = o.tail[:len(o.tail)-overflow]
		}
		o.tail = append(o.tail, p...)
	}
	return n, nil
}

func (o *attemptOutput) networkFailure() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	text := ansiEscape.ReplaceAllString(string(o.tail), "")
	if permanentFailure.MatchString(text) {
		return ""
	}
	// Composer can recover an earlier failed download via another source. When
	// it prints a final exception, classify that block rather than an earlier
	// warning which may be unrelated to the command's eventual failure.
	if headers := exceptionHeader.FindAllStringIndex(text, -1); len(headers) > 0 {
		text = text[headers[len(headers)-1][0]:]
	}
	reason := ""
	for _, failure := range curlFailure.FindAllStringSubmatch(text, -1) {
		switch failure[1] {
		case "6":
			reason = "DNS failure"
		case "7":
			reason = "connection failure"
		case "28":
			reason = "download timeout"
		default:
			// A second, non-transient error (for example certificate validation)
			// must not be hidden by an earlier transient download warning.
			return ""
		}
	}
	return reason
}
