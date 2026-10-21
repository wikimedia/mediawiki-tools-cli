package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	gitlabapi "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	mwoutput "gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"golang.org/x/term"
)

const (
	maxArtifactJobs = 10000
	graphqlPageSize = 100
)

type ciArtifact struct {
	ID        string     `json:"id,omitempty"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Size      int64      `json:"size_bytes"`
	JobID     string     `json:"job_id"`
	JobName   string     `json:"job_name"`
	Ref       string     `json:"ref"`
	CreatedAt *time.Time `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type graphqlInt64 int64

func (value *graphqlInt64) UnmarshalJSON(data []byte) error {
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		text = string(data)
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*value = graphqlInt64(parsed)
	return nil
}

func newCIArtifactsCommand(factory cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifacts",
		Short: "List and manage CI job artifacts",
	}
	cmd.AddCommand(newCIArtifactsListCommand(factory))
	cmd.AddCommand(newCIArtifactsPruneCommand(factory))
	return cmd
}

func newCIArtifactsListCommand(factory cmdutils.Factory) *cobra.Command {
	return newCIArtifactsFilterCommand("list", "List individual CI job artifacts", factory, false)
}

func newCIArtifactsPruneCommand(factory cmdutils.Factory) *cobra.Command {
	return newCIArtifactsFilterCommand("prune", "Delete matching CI job artifacts", factory, true)
}

func newCIArtifactsFilterCommand(use, short string, factory cmdutils.Factory, prune bool) *cobra.Command {
	var sinceText, minSizeText, refPrefix, output string
	var yes bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			since, err := parseArtifactAge(sinceText)
			if err != nil || since <= 0 {
				return fmt.Errorf("invalid --since duration %q (for example, 48h or 30m)", sinceText)
			}
			minSize, err := parseArtifactSize(minSizeText)
			if err != nil {
				return fmt.Errorf("invalid --min-size: %w", err)
			}
			if !prune && output != "table" && output != "json" {
				return fmt.Errorf("invalid --output %q (choose table or json)", output)
			}

			repo, err := factory.BaseRepo()
			if err != nil {
				return err
			}
			client, err := factory.HttpClient()
			if err != nil {
				return err
			}
			project, err := repo.Project(client)
			if err != nil {
				return err
			}

			cutoff := time.Now().Add(-since)
			artifacts, err := listCIArtifacts(cmd, client, project.ID, cutoff, minSize, refPrefix)
			if err != nil {
				return err
			}
			if !prune {
				return outputCIArtifacts(cmd, output, artifacts)
			}
			if len(artifacts) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No artifacts matched the filters.")
				return err
			}
			if err := resolveArtifactIDs(cmd, client, project.PathWithNamespace, artifacts); err != nil {
				return err
			}
			if err := outputCIArtifacts(cmd, "table", artifacts); err != nil {
				return err
			}
			if !yes {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "\nPreview only: no artifacts were deleted. Add --yes to delete these %d individual artifacts. Job logs are not included.\n", len(artifacts))
				return err
			}

			ids := make([]string, len(artifacts))
			for i, artifact := range artifacts {
				ids[i] = artifact.ID
			}
			var response struct {
				Data struct {
					BulkDestroy struct {
						DestroyedCount int      `json:"destroyedCount"`
						DestroyedIDs   []string `json:"destroyedIds"`
						Errors         []string `json:"errors"`
					} `json:"bulkDestroyJobArtifacts"`
				} `json:"data"`
			}
			query, err := bulkDestroyQuery(project.ID, ids)
			if err != nil {
				return err
			}
			if _, err := client.GraphQL.Do(gitlabapi.GraphQLQuery{Query: query}, &response); err != nil {
				return fmt.Errorf("delete artifacts: %w", err)
			}
			if len(response.Data.BulkDestroy.Errors) > 0 {
				return fmt.Errorf("delete artifacts: %s", strings.Join(response.Data.BulkDestroy.Errors, "; "))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted %d of %d selected artifacts.\n", response.Data.BulkDestroy.DestroyedCount, len(ids))
			return err
		},
	}
	cmd.Flags().StringVar(&sinceText, "since", "48h", "Include artifacts from jobs created within this duration (for example, 48h)")
	cmd.Flags().StringVar(&minSizeText, "min-size", "0B", "Minimum size per artifact (for example, 300MiB)")
	cmd.Flags().StringVar(&refPrefix, "ref-prefix", "", "Only include jobs whose ref starts with this prefix")
	if prune {
		cmd.Flags().BoolVar(&yes, "yes", false, "Delete all artifacts matching the filters without prompting")
	} else {
		cmd.Flags().StringVar(&output, "output", "table", "Output format: table or json")
	}
	return cmd
}

func listCIArtifacts(cmd *cobra.Command, client *gitlabapi.Client, projectID int, cutoff time.Time, minSize int64, refPrefix string) ([]ciArtifact, error) {
	options := &gitlabapi.ListJobsOptions{ListOptions: gitlabapi.ListOptions{
		PerPage: 100,
		Page:    1,
	}}
	var result []ciArtifact
	scanned := 0
	progress := startArtifactProgress(cmd.ErrOrStderr(), "Scanning recent CI jobs for artifacts")
	if progress != nil {
		defer progress.finish()
	}
	for job, err := range gitlabapi.Scan2(func(page gitlabapi.PaginationOptionFunc) ([]*gitlabapi.Job, *gitlabapi.Response, error) {
		return client.Jobs.ListProjectJobs(projectID, options, page)
	}) {
		if err != nil {
			return nil, fmt.Errorf("list project jobs: %w", err)
		}
		scanned++
		if scanned > maxArtifactJobs {
			return nil, fmt.Errorf("reached the %d-job safety limit; narrow the time window", maxArtifactJobs)
		}
		if job.CreatedAt == nil {
			return nil, fmt.Errorf("job %d has no creation timestamp; cannot safely apply --since", job.ID)
		}
		// The REST Jobs API documents descending ID order. Job IDs increase
		// over time, so no later page can contain a job within the age window.
		if job.CreatedAt.Before(cutoff) {
			break
		}
		if refPrefix != "" && !strings.HasPrefix(job.Ref, refPrefix) {
			if progress != nil {
				progress.update(int64(scanned), int64(len(result)))
			}
			continue
		}
		for _, artifact := range job.Artifacts {
			if strings.EqualFold(artifact.FileType, "trace") || int64(artifact.Size) < minSize {
				continue
			}
			createdAt := *job.CreatedAt
			result = append(result, ciArtifact{
				Name: artifact.Filename, Type: artifact.FileType, Size: int64(artifact.Size),
				JobID: strconv.Itoa(job.ID), JobName: job.Name, Ref: job.Ref,
				CreatedAt: &createdAt, ExpiresAt: job.ArtifactsExpireAt,
			})
		}
		if progress != nil {
			progress.update(int64(scanned), int64(len(result)))
		}
	}
	return result, nil
}

type artifactProgress struct {
	writer  io.Writer
	jobs    atomic.Int64
	matches atomic.Int64
	stop    chan struct{}
	done    chan struct{}
}

func startArtifactProgress(writer io.Writer, label string) *artifactProgress {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return nil
	}
	progress := &artifactProgress{writer: writer, stop: make(chan struct{}), done: make(chan struct{})}
	fmt.Fprintln(writer, label+"...")
	go func() {
		defer close(progress.done)
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
		frame := 0
		for {
			select {
			case <-progress.stop:
				fmt.Fprintf(writer, "\r\033[KScanned %d jobs; found %d matching artifacts.\n", progress.jobs.Load(), progress.matches.Load())
				return
			case <-ticker.C:
				fmt.Fprintf(writer, "\r\033[K%c %s (%d checked, %d matches)", frames[frame], label, progress.jobs.Load(), progress.matches.Load())
				frame = (frame + 1) % len(frames)
			}
		}
	}()
	return progress
}

func (p *artifactProgress) update(jobs, matches int64) {
	p.jobs.Store(jobs)
	p.matches.Store(matches)
}

func (p *artifactProgress) finish() {
	close(p.stop)
	<-p.done
}

func resolveArtifactIDs(cmd *cobra.Command, client *gitlabapi.Client, projectPath string, artifacts []ciArtifact) error {
	jobArtifacts := make(map[string][]int)
	for index, artifact := range artifacts {
		jobArtifacts[artifact.JobID] = append(jobArtifacts[artifact.JobID], index)
	}
	progress := startArtifactProgress(cmd.ErrOrStderr(), "Resolving matching artifact IDs")
	if progress != nil {
		defer progress.finish()
	}
	jobCount := 0
	resolved := 0
	for jobID, artifactIndexes := range jobArtifacts {
		query := fmt.Sprintf(`query { project(fullPath: %s) { job(id: %s) { artifacts(first: %d) { nodes { id name fileType size } pageInfo { hasNextPage } } } } }`, graphqlString(projectPath), graphqlString("gid://gitlab/Ci::Build/"+jobID), graphqlPageSize)
		var response struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
			Data struct {
				Project *struct {
					Job *struct {
						Artifacts struct {
							Nodes []struct {
								ID       string       `json:"id"`
								Name     string       `json:"name"`
								FileType string       `json:"fileType"`
								Size     graphqlInt64 `json:"size"`
							} `json:"nodes"`
							PageInfo struct {
								HasNextPage bool `json:"hasNextPage"`
							} `json:"pageInfo"`
						} `json:"artifacts"`
					} `json:"job"`
				} `json:"project"`
			} `json:"data"`
		}
		if _, err := client.GraphQL.Do(gitlabapi.GraphQLQuery{Query: query}, &response, gitlabapi.WithContext(cmd.Context())); err != nil {
			return fmt.Errorf("resolve artifact IDs for job %s: %w", jobID, err)
		}
		if len(response.Errors) > 0 {
			return fmt.Errorf("resolve artifact IDs for job %s: %s", jobID, graphQLErrorMessages(response.Errors))
		}
		if response.Data.Project == nil || response.Data.Project.Job == nil {
			return fmt.Errorf("GitLab GraphQL returned no job %s in project %q", jobID, projectPath)
		}
		if response.Data.Project.Job.Artifacts.PageInfo.HasNextPage {
			return fmt.Errorf("job %s has more than %d artifacts; refusing to prune an incomplete list", jobID, graphqlPageSize)
		}
		artifactIDs := make(map[string][]string)
		for _, artifact := range response.Data.Project.Job.Artifacts.Nodes {
			key := artifactMatchKey(artifact.Name, artifact.FileType, int64(artifact.Size))
			artifactIDs[key] = append(artifactIDs[key], artifact.ID)
		}
		for _, index := range artifactIndexes {
			artifact := &artifacts[index]
			key := artifactMatchKey(artifact.Name, artifact.Type, artifact.Size)
			available := artifactIDs[key]
			if len(available) == 0 {
				return fmt.Errorf("could not match artifact %q in job %s to a GraphQL artifact ID", artifact.Name, jobID)
			}
			artifact.ID = available[0]
			artifactIDs[key] = available[1:]
			resolved++
		}
		jobCount++
		if progress != nil {
			progress.update(int64(jobCount), int64(resolved))
		}
	}
	return nil
}

func artifactMatchKey(name, fileType string, size int64) string {
	return fmt.Sprintf("%s\x00%s\x00%d", name, strings.ToUpper(fileType), size)
}

func bulkDestroyQuery(projectID int, ids []string) (string, error) {
	projectGID, err := json.Marshal(fmt.Sprintf("gid://gitlab/Project/%d", projectID))
	if err != nil {
		return "", err
	}
	artifactIDs, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`mutation { bulkDestroyJobArtifacts(input: { projectId: %s, ids: %s }) { destroyedCount destroyedIds errors } }`, projectGID, artifactIDs), nil
}

func graphqlString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func graphQLErrorMessages(errors []struct {
	Message string `json:"message"`
}) string {
	messages := make([]string, len(errors))
	for index, graphQLError := range errors {
		messages[index] = graphQLError.Message
	}
	return strings.Join(messages, "; ")
}

func parseArtifactSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("size cannot be empty")
	}
	index := 0
	for index < len(value) && (value[index] == '.' || value[index] >= '0' && value[index] <= '9') {
		index++
	}
	number, err := strconv.ParseFloat(value[:index], 64)
	if err != nil || number < 0 || math.IsInf(number, 0) || math.IsNaN(number) {
		return 0, fmt.Errorf("expected a non-negative size such as 300MiB")
	}
	unit := strings.ToLower(strings.TrimSpace(value[index:]))
	multiplier, ok := map[string]int64{
		"": 1, "b": 1,
		"kb": 1000, "kib": 1 << 10,
		"mb": 1000 * 1000, "mib": 1 << 20,
		"gb": 1000 * 1000 * 1000, "gib": 1 << 30,
		"tb": 1000 * 1000 * 1000 * 1000, "tib": 1 << 40,
	}[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported unit %q; use B, KB, KiB, MB, MiB, GB, GiB, TB or TiB", unit)
	}
	bytes := number * float64(multiplier)
	if bytes > math.MaxInt64 {
		return 0, fmt.Errorf("size is too large")
	}
	return int64(bytes), nil
}

func parseArtifactAge(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(value, "d"), 64)
		if err != nil || days <= 0 || math.IsInf(days, 0) || math.IsNaN(days) || days > float64(math.MaxInt64)/(float64(24*time.Hour)) {
			return 0, fmt.Errorf("invalid duration")
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(value)
}

func outputCIArtifacts(cmd *cobra.Command, output string, artifacts []ciArtifact) error {
	if output == "json" {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(artifacts)
	}
	if len(artifacts) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No artifacts matched the filters.")
		return err
	}
	table := mwoutput.NewTable([]interface{}{"JOB ID", "JOB", "REF", "ARTIFACT", "TYPE", "MiB", "CREATED", "EXPIRES"}, nil)
	for _, artifact := range artifacts {
		created, expires := "-", "-"
		if artifact.CreatedAt != nil {
			created = artifact.CreatedAt.Format(time.RFC3339)
		}
		if artifact.ExpiresAt != nil {
			expires = artifact.ExpiresAt.Format(time.RFC3339)
		}
		table.AddRow(artifact.JobID, artifact.JobName, artifact.Ref, artifact.Name, strings.ToLower(artifact.Type), fmt.Sprintf("%.1f", float64(artifact.Size)/(1<<20)), created, expires)
	}
	table.Print(cmd.OutOrStdout())
	return nil
}
