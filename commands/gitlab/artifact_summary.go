package gitlab

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/mwcli"
	mwoutput "gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func newCIArtifactsSummaryCommand(factory mwcli.Factory) *cobra.Command {
	var sinceText, minSizeText, refPrefix, output string
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarize artifact storage by job name and artifact type",
		Long:  "Summarize currently stored artifacts across the project's entire job history, including retried jobs. Show totals and breakdowns by job name and GitLab artifact type, largest first. Job logs are excluded. Sizes describe stored files, not the contents of archives. This command never deletes artifacts.",
		Args:  cobra.NoArgs,
		Example: "```shell\nmw gitlab ci artifacts summary\n" +
			"mw gitlab ci artifacts summary --since 7d --ref-prefix refs/merge-requests/\n" +
			"mw gitlab ci artifacts summary --output json\n```",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var cutoff time.Time
			if sinceText != "all" {
				since, err := parseArtifactAge(sinceText)
				if err != nil || since <= 0 {
					return fmt.Errorf("invalid --since duration %q (choose all or a duration such as 48h)", sinceText)
				}
				cutoff = time.Now().Add(-since)
			}
			minSize, err := parseArtifactSize(minSizeText)
			if err != nil {
				return fmt.Errorf("invalid --min-size: %w", err)
			}
			if output != "table" && output != "json" {
				return fmt.Errorf("invalid --output %q (choose table or json)", output)
			}
			repo, err := factory.BaseRepo()
			if err != nil {
				return err
			}
			client, err := factory.GitLabClient()
			if err != nil {
				return err
			}
			project, err := repo.Project(client)
			if err != nil {
				return err
			}
			summary := newArtifactSummary()
			if err := scanCIArtifacts(cmd, client, project.ID, cutoff, minSize, refPrefix, 0, summary.add); err != nil {
				return err
			}
			return outputArtifactSummary(cmd, output, summary.result())
		},
	}
	cmd.Flags().StringVar(&sinceText, "since", "all", "Include jobs created within this duration, or all for the entire history")
	cmd.Flags().StringVar(&minSizeText, "min-size", "0B", "Minimum size per artifact (for example, 300MiB)")
	cmd.Flags().StringVar(&refPrefix, "ref-prefix", "", "Only include jobs whose ref starts with this prefix")
	cmd.Flags().StringVar(&output, "output", "table", "Output format: table or json")
	return cmd
}

type artifactUsage struct {
	Jobs      int   `json:"jobs"`
	Artifacts int   `json:"artifacts"`
	SizeBytes int64 `json:"size_bytes"`
}

type artifactUsageGroup struct {
	Name string `json:"name"`
	artifactUsage
}

type artifactStorageSummary struct {
	Total  artifactUsage        `json:"total"`
	ByJob  []artifactUsageGroup `json:"by_job"`
	ByType []artifactUsageGroup `json:"by_type"`
}

type artifactUsageAccumulator struct {
	artifactUsage
	jobs map[string]bool
}

func (a *artifactUsageAccumulator) add(artifact ciArtifact) {
	if a.jobs == nil {
		a.jobs = make(map[string]bool)
	}
	if !a.jobs[artifact.JobID] {
		a.jobs[artifact.JobID] = true
		a.Jobs++
	}
	a.Artifacts++
	a.SizeBytes += artifact.Size
}

type artifactSummary struct {
	total  artifactUsageAccumulator
	byJob  map[string]*artifactUsageAccumulator
	byType map[string]*artifactUsageAccumulator
}

func newArtifactSummary() *artifactSummary {
	return &artifactSummary{
		byJob:  make(map[string]*artifactUsageAccumulator),
		byType: make(map[string]*artifactUsageAccumulator),
	}
}

func (s *artifactSummary) add(artifact ciArtifact) error {
	if artifact.Size < 0 || artifact.Size > math.MaxInt64-s.total.SizeBytes {
		return fmt.Errorf("invalid or overflowing artifact size for job %s artifact %q", artifact.JobID, artifact.Name)
	}
	s.total.add(artifact)
	fileType := strings.ToLower(artifact.Type)
	if fileType == "" {
		fileType = "unknown"
	}
	for _, group := range []struct {
		values map[string]*artifactUsageAccumulator
		name   string
	}{{s.byJob, artifact.JobName}, {s.byType, fileType}} {
		if group.values[group.name] == nil {
			group.values[group.name] = &artifactUsageAccumulator{}
		}
		group.values[group.name].add(artifact)
	}
	return nil
}

func sortedArtifactUsage(values map[string]*artifactUsageAccumulator) []artifactUsageGroup {
	groups := make([]artifactUsageGroup, 0, len(values))
	for name, usage := range values {
		groups = append(groups, artifactUsageGroup{Name: name, artifactUsage: usage.artifactUsage})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].SizeBytes == groups[j].SizeBytes {
			return groups[i].Name < groups[j].Name
		}
		return groups[i].SizeBytes > groups[j].SizeBytes
	})
	return groups
}

func (s *artifactSummary) result() artifactStorageSummary {
	return artifactStorageSummary{
		Total: s.total.artifactUsage, ByJob: sortedArtifactUsage(s.byJob), ByType: sortedArtifactUsage(s.byType),
	}
}

func outputArtifactSummary(cmd *cobra.Command, output string, summary artifactStorageSummary) error {
	if output == "json" {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Total: %s in %d artifacts across %d jobs (job logs excluded).\n", formatArtifactSize(summary.Total.SizeBytes), summary.Total.Artifacts, summary.Total.Jobs); err != nil {
		return err
	}
	for _, breakdown := range []struct {
		label  string
		groups []artifactUsageGroup
	}{{"JOB NAME", summary.ByJob}, {"ARTIFACT TYPE", summary.ByType}} {
		if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
			return err
		}
		table := mwoutput.NewTable([]interface{}{breakdown.label, "JOBS", "ARTIFACTS", "SIZE"}, nil)
		for _, group := range breakdown.groups {
			table.AddRow(group.Name, group.Jobs, group.Artifacts, formatArtifactSize(group.SizeBytes))
		}
		table.Print(cmd.OutOrStdout())
	}
	return nil
}
