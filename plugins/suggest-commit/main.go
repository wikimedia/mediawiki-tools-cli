//go:build llama && cgo

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

const (
	maxDiffBytes = 8 * 1024
	// Leave room for the instructions even when the excerpt tokenizes densely.
	maxExcerptBytes = 3 * 1024
	maxSuggestions  = 5
)

func main() {
	var model string
	var last bool
	var count int
	command := &cobra.Command{
		Use:          "wm suggest-commit",
		Short:        "Draft a commit subject using a local model",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if count < 1 || count > maxSuggestions {
				return fmt.Errorf("--count must be between 1 and %d", maxSuggestions)
			}
			diff, err := readDiff(cmd.ErrOrStderr(), last)
			if err != nil {
				return err
			}

			request := "Write a Git commit subject for the main change."
			if count > 1 {
				request = fmt.Sprintf("Write %d alternative Git commit subjects, one per line. "+
					"Rephrase the same main change each time. Use only facts supported by the diff.", count)
			}
			prompt := "<|im_start|>user\nHere is a Git diff:\n\n" +
				strings.ReplaceAll(string(diff), "<|", "[|") +
				"\n" + request + " Use an imperative verb. " +
				"Aim for 50–70 characters and stay under 80. Lead with the change itself. " +
				"Use a component prefix only when it clarifies the area. " +
				"Return only the requested plain-text subjects, without final periods, URLs or task IDs.<|im_end|>\n<|im_start|>assistant\n"
			draft, err := generate(model, prompt, outputTokensPerSubject*count)
			if err != nil {
				return err
			}
			subjects := subjectsFromDraft(draft, count)
			if len(subjects) == 0 {
				return fmt.Errorf("the model returned no commit subjects")
			}
			if len(subjects) < count {
				fmt.Fprintf(cmd.ErrOrStderr(), "The model returned %d of %d distinct suggestions.\n", len(subjects), count)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(subjects, "\n"))
			return err
		},
	}
	command.Flags().StringVar(&model, "model", os.Getenv("WM_COMMIT_MODEL"), "Path to a ChatML instruct model in GGUF format")
	command.Flags().BoolVar(&last, "last", false, "Use the latest commit (HEAD) instead of staged changes")
	command.Flags().IntVar(&count, "count", 1, fmt.Sprintf("Number of suggested subjects (1–%d)", maxSuggestions))
	if model == "" {
		_ = command.MarkFlagRequired("model")
	}
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

var listMarker = regexp.MustCompile(`^(?:[0-9]+[.)]\s+|[-*]\s*)`)

func subjectsFromDraft(draft string, count int) []string {
	subjects := make([]string, 0, count)
	seen := make(map[string]bool, count)
	for line := range strings.SplitSeq(draft, "\n") {
		line = strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line))
		if line == "" || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "diff --git ") {
			continue
		}
		line = listMarker.ReplaceAllString(line, "")
		if strings.HasSuffix(line, ":") {
			continue
		}
		subject := strings.TrimSpace(strings.Trim(strings.ReplaceAll(line, "`", ""), "\""))
		subject = strings.TrimSpace(strings.TrimRight(subject, "."))
		key := strings.ToLower(subject)
		if subject == "" || seen[key] {
			continue
		}
		seen[key] = true
		subjects = append(subjects, subject)
		if len(subjects) == count {
			break
		}
	}
	return subjects
}

func readDiff(stderr io.Writer, last bool) ([]byte, error) {
	args := []string{"diff", "--cached"}
	source := "staged diff"
	emptyMessage := "no staged changes"
	if last {
		args = []string{
			"show", "--format=", "--no-notes", "--diff-merges=first-parent",
			"--stat=100,60,15", "--patch", "HEAD",
		}
		source = "latest commit diff"
		emptyMessage = "the latest commit has no changes"
	}
	args = append(args, "--no-ext-diff", "--no-textconv", "--no-color", "--unified=1", "--")
	git := exec.Command("git", args...)
	git.Stderr = stderr
	stdout, err := git.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := git.Start(); err != nil {
		return nil, err
	}
	diff, readErr := io.ReadAll(io.LimitReader(stdout, maxDiffBytes+1))
	if readErr != nil || len(diff) > maxDiffBytes {
		_ = git.Process.Kill()
	}
	waitErr := git.Wait()
	switch {
	case readErr != nil:
		return nil, fmt.Errorf("read %s: %w", source, readErr)
	case len(diff) > maxDiffBytes && last:
		excerpt := diff[:maxExcerptBytes]
		// The file summary comes first. End the excerpt at a complete line.
		if end := bytes.LastIndexByte(excerpt, '\n'); end >= 0 {
			excerpt = excerpt[:end+1]
		}
		_, _ = fmt.Fprintln(stderr, "The latest commit is large. Drafting from the file summary and a diff excerpt.")
		return append(excerpt, "\n[Remaining diff omitted]\n"...), nil
	case len(diff) > maxDiffBytes:
		return nil, fmt.Errorf("%s exceeds %d bytes", source, maxDiffBytes)
	case waitErr != nil:
		return nil, fmt.Errorf("read %s: %w", source, waitErr)
	case len(bytes.TrimSpace(diff)) == 0:
		return nil, fmt.Errorf("%s", emptyMessage)
	default:
		return diff, nil
	}
}
