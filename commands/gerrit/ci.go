package gerrit

import (
	"context"
	"fmt"
	"io"
	"os"
	ossignal "os/signal"
	"slices"
	"strings"
	"time"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func newCICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ci",
		Short:   "Inspect Wikimedia CI for Gerrit changes",
		Example: "ci status 1372985\nci logs 1372985 --job mediawiki-core-phan-only-php83",
	}
	cmd.AddCommand(newCIStatusCmd(), newCILogsCmd())
	return cmd
}

func ciReadCommand(cmd *cobra.Command, patchset *int) *cobra.Command {
	accesspolicy.Support(cmd)
	commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic},
	})
	cmd.Flags().IntVar(patchset, "patchset", 0, "Inspect this patchset")
	cmd.ValidArgsFunction = cobra.NoFileCompletions
	return cmd
}

func newCIStatusCmd() *cobra.Command {
	var patchset int
	var watch bool
	var out output.Output
	cmd := &cobra.Command{
		Use:     "status [change]",
		Short:   "Show live and reported CI jobs for a Gerrit patchset",
		Long:    "Pass a Gerrit change number or URL, or run from a checkout whose HEAD matches an uploaded patchset. Status combines the public Zuul queue with the latest completed report for each pipeline.",
		Example: "status 1372985 --patchset 2\nstatus --watch\nstatus 1372985 --output json",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCIOutput(out); err != nil {
				return err
			}
			ctx := cmd.Context()
			if watch {
				var stop context.CancelFunc
				ctx, stop = ossignal.NotifyContext(ctx, os.Interrupt)
				defer stop()
			}
			client, target, err := prepareCI(ctx, cmd, args, patchset)
			if err != nil {
				return err
			}
			return printCIStatus(ctx, cmd, client, target, out, watch)
		},
	}
	cmd.Flags().BoolVar(&watch, "watch", false, "Print status changes until interrupted")
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.TableType), output.WithFilterFlagDisabled(),
		output.WithTableBinding(&output.TableBinding{}))
	return ciReadCommand(cmd, &patchset)
}

func validateCIOutput(out output.Output) error {
	switch output.Type(out.Type) {
	case output.TableType:
		if out.Format != "" {
			return fmt.Errorf("--format requires JSON or jq output")
		}
	case output.JSONType, output.JQType:
		return output.ValidateJSONFormat(out.Format)
	default:
		return fmt.Errorf("unsupported output %q, choose table, json or jq", out.Type)
	}
	return nil
}

func prepareCI(ctx context.Context, cmd *cobra.Command, args []string, patchset int) (*gogerrit.Client, ciTarget, error) {
	ctx, cancel := context.WithTimeout(ctx, gerritRequestTimeout)
	defer cancel()
	if cmd.Flags().Changed("patchset") && patchset < 1 {
		return nil, ciTarget{}, fmt.Errorf("--patchset must be positive")
	}
	target, err := selectCITarget(ctx, args, patchset)
	if err != nil {
		return nil, ciTarget{}, err
	}
	client, err := newGerritClient(ctx)
	if err != nil {
		return nil, ciTarget{}, err
	}
	if err := target.resolveChange(ctx, client); err != nil {
		return nil, ciTarget{}, err
	}
	return client, target, nil
}

func printCIStatus(ctx context.Context, cmd *cobra.Command, client *gogerrit.Client, target ciTarget, out output.Output, watch bool) error {
	var previous ciSnapshot
	var ticker *time.Ticker
	if watch {
		ticker = time.NewTicker(5 * time.Second)
		defer ticker.Stop()
	}
	for {
		request, cancel := context.WithTimeout(ctx, gerritRequestTimeout)
		snapshot, queueErr, err := captureCIStatus(request, client, target)
		cancel()
		if watch && ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		if queueErr != nil && (previous.Change == 0 || previous.QueueAvailable) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Zuul live status unavailable: %v\n", queueErr)
		}
		if previous.Change != snapshot.Change || previous.Patchset != snapshot.Patchset ||
			previous.QueueAvailable != snapshot.QueueAvailable || !slices.Equal(previous.Jobs, snapshot.Jobs) {
			if err := renderCISnapshot(cmd.OutOrStdout(), snapshot, out); err != nil {
				return err
			}
			previous = snapshot
		}
		if !watch {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func renderCISnapshot(writer io.Writer, snapshot ciSnapshot, out output.Output) error {
	if out.Type != string(output.TableType) {
		return output.PrintJSONChecked(snapshot, out.Format, out.Type == string(output.JQType), writer)
	}
	if _, err := fmt.Fprintf(writer, "Change %d, patchset %d (%s)\n", snapshot.Change, snapshot.Patchset, snapshot.Project); err != nil {
		return err
	}
	if len(snapshot.Jobs) == 0 {
		message := "No CI jobs reported or in the queue for this patchset."
		if !snapshot.QueueAvailable {
			message = "No CI jobs reported for this patchset."
		}
		_, err := fmt.Fprintln(writer, message)
		return err
	}
	table := output.Table{}
	table.AddHeadingsS("PIPELINE", "JOB", "STATUS", "SOURCE", "DURATION", "BUILD")
	for _, job := range snapshot.Jobs {
		source := "reported"
		if job.Active {
			source = "live"
		}
		status := job.Status
		if job.NonVoting {
			status += " (non-voting)"
		}
		table.AddRowS(job.Pipeline, job.Name, status, source, job.Duration, job.BuildURL)
	}
	table.Print(writer)
	return nil
}

func newCILogsCmd() *cobra.Command {
	var patchset int
	var jobName, pipeline string
	cmd := &cobra.Command{
		Use:   "logs [change]",
		Short: "Print a Jenkins job's console log",
		Long: "Pass a Gerrit change number or URL, or run from a checkout whose HEAD matches an uploaded patchset. " +
			"Select a job by name, or omit --job when one failed job has a log.",
		Example: "logs 1372985 --patchset 2 --job mediawiki-core-phan-only-php83\nlogs --pipeline test",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, target, err := prepareCI(ctx, cmd, args, patchset)
			if err != nil {
				return err
			}
			request, cancel := context.WithTimeout(ctx, gerritRequestTimeout)
			snapshot, queueErr, err := captureCIStatus(request, client, target)
			cancel()
			if err != nil {
				return err
			}
			job, err := chooseCILog(snapshot, jobName, pipeline)
			if err != nil {
				if queueErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Zuul live status unavailable: %v\n", queueErr)
				}
				return err
			}
			logContext, cancelLog := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelLog()
			resp, err := getCIIntegration(logContext, job.BuildURL+"consoleText")
			if err != nil {
				return fmt.Errorf("read %s console log: %w", job.Name, err)
			}
			defer resp.Body.Close()
			_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
			return err
		},
	}
	cmd.Flags().StringVar(&jobName, "job", "", "Jenkins job name")
	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Zuul pipeline name")
	return ciReadCommand(cmd, &patchset)
}

func chooseCILog(snapshot ciSnapshot, name, pipeline string) (ciJob, error) {
	var matches []ciJob
	for _, job := range snapshot.Jobs {
		if job.BuildURL == "" || (pipeline != "" && job.Pipeline != pipeline) {
			continue
		}
		if (name == "" && !job.failed()) || (name != "" && job.Name != name) {
			continue
		}
		index := slices.IndexFunc(matches, func(other ciJob) bool {
			return other.Pipeline == job.Pipeline && other.Name == job.Name
		})
		if index < 0 {
			matches = append(matches, job)
		} else if job.Active {
			matches[index] = job
		}
	}
	switch len(matches) {
	case 0:
		if name != "" {
			return ciJob{}, fmt.Errorf("no console log for job %q on patchset %d", name, snapshot.Patchset)
		}
		return ciJob{}, fmt.Errorf("no failed job with a console log on patchset %d. Run status and select a job with --job", snapshot.Patchset)
	case 1:
		return matches[0], nil
	default:
		var choices []string
		for _, job := range matches {
			choices = append(choices, fmt.Sprintf("%s (%s)", job.Name, job.Pipeline))
		}
		return ciJob{}, fmt.Errorf("multiple jobs have logs. Select one with --job and, if needed, --pipeline: %s", strings.Join(choices, ", "))
	}
}
