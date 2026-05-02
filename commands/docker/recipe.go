package docker

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd/recipe"
	filesutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

const recipeRuntimeStateFileName = ".mwcli-recipe-state.json"
const recipeManagedComposeHeader = "# Managed by mwcli recipe"

type recipeRuntimeState struct {
	EnvKeys        []string `json:"envKeys"`
	ComposeFiles   []string `json:"composeFiles"`
	JobRunnerSites []string `json:"jobRunnerSites"`
}

func NewRecipeCmd() *cobra.Command {
	var recipeFile string
	var recipeURL string
	var recipeName string
	var dryRun bool
	var skipCode bool
	var skipServices bool
	var skipSites bool
	var skipMaintenance bool
	var skipPatches bool

	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Apply a YAML recipe to set up a complete dev environment",
		Long:  "Apply a YAML recipe to set up services, checkout code, install sites, apply LocalSettings config, and run maintenance commands.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := mwdd.DefaultForUser()
			m.EnsureReady()

			if len(args) > 0 {
				recipeName = args[0]
			}

			spec, err := loadRecipe(recipeFile, recipeURL, recipeName, m)
			if err != nil {
				return err
			}

			fmt.Printf("Applying recipe %q (%s)\n", spec.Name, spec.Version)
			if dryRun {
				fmt.Println("Dry run mode enabled. No changes will be made.")
			}

			// Ensure recipe-scoped state from previous runs does not leak into this run.
			if err := cleanupRecipeRuntimeState(m, dryRun); err != nil {
				return err
			}

			appliedEnvKeys := []string{}

			for k, v := range spec.Env {
				appliedEnvKeys = append(appliedEnvKeys, k)
				if dryRun {
					fmt.Printf("[dry-run] set env %s=%s\n", k, v)
					continue
				}
				m.Env().Set(k, v)
			}

			if m.Env().Missing("MEDIAWIKI_VOLUMES_CODE") {
				guess := mediawiki.GuessMediaWikiDirectoryBasedOnContext()
				if dryRun {
					fmt.Printf("[dry-run] set MEDIAWIKI_VOLUMES_CODE=%s\n", guess)
				} else {
					m.Env().Set("MEDIAWIKI_VOLUMES_CODE", guess)
				}
			}

			mediaWikiPath := m.Env().Get("MEDIAWIKI_VOLUMES_CODE")
			if mediaWikiPath == "" {
				return fmt.Errorf("MEDIAWIKI_VOLUMES_CODE is empty")
			}

			thisMW, _ := mediawiki.ForDirectory(mediaWikiPath)

			customComposeFile := ""
			if strings.TrimSpace(spec.CustomCompose.Content) != "" {
				customComposeFile = customComposeFileName(spec.CustomCompose)
				if err := writeCustomComposeFile(m, spec.CustomCompose, dryRun); err != nil {
					return err
				}
			}

			if !dryRun {
				if err := saveRecipeRuntimeState(m, recipeRuntimeState{
					EnvKeys:        appliedEnvKeys,
					ComposeFiles:   nonEmptyStrings([]string{customComposeFile}),
					JobRunnerSites: spec.JobRunner.Sites,
				}); err != nil {
					return err
				}
			}

			if !skipCode {
				if err := applyCodeCheckout(spec, thisMW, dryRun); err != nil {
					return err
				}
			}

			requiredServices := servicesRequiredBySites(spec.Sites)
			for _, svc := range requiredServices {
				if !hasService(spec.Services, svc) {
					spec.Services = append(spec.Services, recipe.Service{Name: svc, State: "started"})
				}
			}
			if len(spec.JobRunner.Sites) > 0 && !hasService(spec.Services, "mediawiki-jobrunner") {
				spec.Services = append(spec.Services, recipe.Service{Name: "mediawiki-jobrunner", State: "started"})
			}

			if err := applyJobRunnerSites(m, spec.JobRunner.Sites, dryRun); err != nil {
				return err
			}

			if !skipServices {
				if err := applyServices(spec.Services, m, dryRun); err != nil {
					return err
				}
			}

			if err := syncComposerLocalAndUpdate(m, thisMW, spec.Code, dryRun); err != nil {
				return err
			}

			if err := removeManagedRecipeLocalSettings(thisMW, dryRun); err != nil {
				return err
			}

			// Apply LocalSettings BEFORE installing sites so that update.php
			// (run inside installSite) picks up extension configuration.
			if err := applyLocalSettings(thisMW, spec.Name, spec.LocalSettings, dryRun); err != nil {
				return err
			}

			if !skipSites {
				for _, site := range spec.Sites {
					if err := installSite(m, thisMW, site, dryRun); err != nil {
						return err
					}
				}
			}

			if !skipMaintenance {
				if err := runMaintenanceSteps(m, spec.Maintenance, dryRun); err != nil {
					return err
				}
			}

			if err := applyContent(m, spec.Sites, spec.Content, dryRun); err != nil {
				return err
			}

			if !skipPatches {
				if err := applyPatches(mediaWikiPath, spec.Patches, dryRun); err != nil {
					return err
				}
			}

			outputDetails := map[string]string{
				"Recipe": spec.Name,
			}
			if len(spec.Sites) > 0 {
				siteURLs := []string{}
				for _, site := range spec.Sites {
					siteURLs = append(siteURLs, "http://"+site.DBName+".mediawiki.local.wmftest.net:"+m.Env().Get("PORT")+"/wiki/Main_Page")
				}
				outputDetails["Sites"] = strings.Join(siteURLs, "\n")
			}
			if spec.Description != "" {
				outputDetails["Info"] = spec.Description
			}
			cmdgloss.PrintThreePartBlock(
				cmdgloss.SuccessHeading("Recipe applied successfully"),
				outputDetails,
				"Run `mw dev status` to see running services.",
			)
			return nil
		},
	}

	cmd.Flags().StringVarP(&recipeFile, "file", "f", "", "Path to recipe YAML file")
	cmd.Flags().StringVar(&recipeURL, "url", "", "URL to recipe YAML file")
	cmd.Flags().StringVarP(&recipeName, "name", "n", "", "Name of a recipe in the local extracted recipes directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned operations without changing anything")
	cmd.Flags().BoolVar(&skipCode, "skip-code", false, "Skip code checkout phase")
	cmd.Flags().BoolVar(&skipServices, "skip-services", false, "Skip service create/start phase")
	cmd.Flags().BoolVar(&skipSites, "skip-sites", false, "Skip site installation phase")
	cmd.Flags().BoolVar(&skipMaintenance, "skip-maintenance", false, "Skip maintenance commands phase")
	cmd.Flags().BoolVar(&skipPatches, "skip-patches", false, "Skip patch apply phase")
	cmd.MarkFlagsMutuallyExclusive("file", "url", "name")

	cmd.AddCommand(newRecipeValidateCmd())
	return cmd
}

func recipeRuntimeStatePath(m mwdd.MWDD) string {
	return filepath.Clean(filepath.Join(m.Directory(), recipeRuntimeStateFileName))
}

func loadRecipeRuntimeState(m mwdd.MWDD) (recipeRuntimeState, error) {
	path := recipeRuntimeStatePath(m)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return recipeRuntimeState{}, nil
	}
	if err != nil {
		return recipeRuntimeState{}, err
	}

	state := recipeRuntimeState{}
	if err := json.Unmarshal(b, &state); err != nil {
		return recipeRuntimeState{}, err
	}
	return state, nil
}

func saveRecipeRuntimeState(m mwdd.MWDD, state recipeRuntimeState) error {
	path := recipeRuntimeStatePath(m)
	merged := mergeRecipeRuntimeState(state)
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func cleanupRecipeRuntimeState(m mwdd.MWDD, dryRun bool) error {
	state, err := loadRecipeRuntimeState(m)
	if err != nil {
		return err
	}

	for _, key := range uniqueStrings(nonEmptyStrings(state.EnvKeys)) {
		if dryRun {
			fmt.Printf("[dry-run] unset env %s\n", key)
			continue
		}
		m.Env().Delete(key)
	}

	for _, composeFile := range uniqueStrings(nonEmptyStrings(state.ComposeFiles)) {
		path := filepath.Clean(filepath.Join(m.Directory(), composeFile))
		if dryRun {
			fmt.Printf("[dry-run] remove compose file %s\n", path)
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if len(state.JobRunnerSites) > 0 {
		jobRunnerSitesFile := filepath.Clean(filepath.Join(m.Directory(), "mediawiki", "jobrunner-sites"))
		for _, site := range uniqueStrings(nonEmptyStrings(state.JobRunnerSites)) {
			if dryRun {
				fmt.Printf("[dry-run] remove jobrunner site %s from %s\n", site, jobRunnerSitesFile)
				continue
			}
			filesutil.RemoveAllLinesMatching(site, jobRunnerSitesFile)
		}
	}

	legacyComposeCleaned, err := cleanupLegacyRecipeComposeFiles(m, dryRun)
	if err != nil {
		return err
	}
	if legacyComposeCleaned {
		for _, key := range []string{"MEDIAWIKI_DEFAULT_DBNAME", "CXSERVER_VOLUMES_CODE"} {
			if dryRun {
				fmt.Printf("[dry-run] unset legacy env %s\n", key)
				continue
			}
			m.Env().Delete(key)
		}
	}

	if dryRun {
		fmt.Printf("[dry-run] remove recipe runtime state file %s\n", recipeRuntimeStatePath(m))
		return nil
	}
	if err := os.Remove(recipeRuntimeStatePath(m)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func cleanupLegacyRecipeComposeFiles(m mwdd.MWDD, dryRun bool) (bool, error) {
	entries, err := os.ReadDir(m.Directory())
	if err != nil {
		return false, err
	}

	cleaned := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !(name == "custom.yml" || strings.HasPrefix(name, "custom-")) {
			continue
		}
		if filepath.Ext(name) != ".yml" && filepath.Ext(name) != ".yaml" {
			continue
		}

		path := filepath.Clean(filepath.Join(m.Directory(), name))
		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return cleaned, readErr
		}
		content := string(contentBytes)
		isManaged := strings.Contains(content, recipeManagedComposeHeader)
		isLegacyCX := strings.Contains(content, "CXSERVER_VOLUMES_CODE") && strings.Contains(content, "cxserver")
		if !isManaged && !isLegacyCX {
			continue
		}

		if dryRun {
			fmt.Printf("[dry-run] remove legacy compose file %s\n", path)
			cleaned = true
			continue
		}
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return cleaned, rmErr
		}
		cleaned = true
	}

	return cleaned, nil
}

func mergeRecipeRuntimeState(state recipeRuntimeState) recipeRuntimeState {
	state.EnvKeys = uniqueStrings(nonEmptyStrings(state.EnvKeys))
	state.ComposeFiles = uniqueStrings(nonEmptyStrings(state.ComposeFiles))
	state.JobRunnerSites = uniqueStrings(nonEmptyStrings(state.JobRunnerSites))
	return state
}

func nonEmptyStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func newRecipeValidateCmd() *cobra.Command {
	var recipeFile string
	var recipeURL string
	var recipeName string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a recipe YAML file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := mwdd.DefaultForUser()
			m.EnsureReady()

			if len(args) > 0 {
				recipeName = args[0]
			}

			spec, err := loadRecipe(recipeFile, recipeURL, recipeName, m)
			if err != nil {
				return err
			}
			fmt.Printf("Recipe valid: %q (%s)\n", spec.Name, spec.Version)
			fmt.Printf("Code checkout: core=%v, extensions=%d, skins=%d\n", spec.Code.Core, len(spec.Code.Extensions), len(spec.Code.Skins))
			fmt.Printf("Services=%d, Sites=%d, Maintenance steps=%d, Patches=%d\n", len(spec.Services), len(spec.Sites), len(spec.Maintenance), len(spec.Patches))
			return nil
		},
	}

	cmd.Flags().StringVarP(&recipeFile, "file", "f", "", "Path to recipe YAML file")
	cmd.Flags().StringVar(&recipeURL, "url", "", "URL to recipe YAML file")
	cmd.Flags().StringVarP(&recipeName, "name", "n", "", "Name of a recipe in the local extracted recipes directory")
	cmd.MarkFlagsMutuallyExclusive("file", "url", "name")
	return cmd
}

func loadRecipe(recipeFile string, recipeURL string, recipeName string, m mwdd.MWDD) (recipe.Spec, error) {
	if recipeFile == "" && recipeURL == "" && recipeName == "" {
		return recipe.Spec{}, fmt.Errorf("you must provide either --file, --url, or a recipe name")
	}

	var content []byte
	if recipeFile != "" {
		resolved, err := filepath.Abs(recipeFile)
		if err != nil {
			return recipe.Spec{}, err
		}
		b, err := os.ReadFile(filepath.Clean(resolved))
		if err != nil {
			return recipe.Spec{}, err
		}
		content = b
	}

	if recipeURL != "" {
		resp, err := http.Get(recipeURL) // #nosec G107
		if err != nil {
			return recipe.Spec{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return recipe.Spec{}, fmt.Errorf("failed to download recipe: %s", resp.Status)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return recipe.Spec{}, err
		}
		content = b
	}

	if recipeName != "" {
		name := strings.TrimSpace(recipeName)
		candidates := []string{}
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			candidates = append(candidates, name)
		} else {
			candidates = append(candidates, name+".yaml", name+".yml")
		}

		var firstErr error
		for _, candidate := range candidates {
			resolved := filepath.Clean(filepath.Join(m.Directory(), "recipes", candidate))
			b, err := os.ReadFile(resolved)
			if err == nil {
				content = b
				break
			}
			if firstErr == nil {
				firstErr = err
			}
		}

		if len(content) == 0 {
			return recipe.Spec{}, fmt.Errorf("failed to load local recipe %q from %s/recipes: %w", recipeName, m.Directory(), firstErr)
		}
	}

	return recipe.Parse(content)
}

func writeCustomComposeFile(m mwdd.MWDD, custom recipe.CustomCompose, dryRun bool) error {
	path := filepath.Clean(filepath.Join(m.Directory(), customComposeFileName(custom)))
	if dryRun {
		fmt.Printf("[dry-run] write custom compose file %s\n", path)
		return nil
	}
	content := strings.TrimSpace(custom.Content)
	if !strings.HasPrefix(content, recipeManagedComposeHeader) {
		content = recipeManagedComposeHeader + "\n" + content
	}
	return os.WriteFile(path, []byte(content+"\n"), 0o644)
}

func customComposeFileName(custom recipe.CustomCompose) string {
	name := strings.TrimSpace(custom.Name)
	if name == "" {
		name = "custom"
	}
	return name + ".yml"
}

func applyCodeCheckout(spec recipe.Spec, thisMW mediawiki.MediaWiki, dryRun bool) error {
	gerritInteractionType := spec.Source.GerritInteractionType
	gerritUsername := spec.Source.GerritUsername
	configState := config.State()

	if gerritInteractionType == "" {
		gerritInteractionType = strings.TrimSpace(configState.EffectiveKoanf.String("gerrit.interaction_type"))
		if gerritInteractionType == "" {
			gerritInteractionType = strings.TrimSpace(configState.OnDiskKoanf.String("gerrit.interaction_type"))
		}
		if gerritInteractionType == "" {
			gerritInteractionType = strings.TrimSpace(configState.Effective.Gerrit.InteractionType)
		}
	}
	if gerritUsername == "" {
		gerritUsername = strings.TrimSpace(configState.EffectiveKoanf.String("gerrit.username"))
		if gerritUsername == "" {
			gerritUsername = strings.TrimSpace(configState.OnDiskKoanf.String("gerrit.username"))
		}
		if gerritUsername == "" {
			gerritUsername = strings.TrimSpace(configState.Effective.Gerrit.Username)
		}
	}

	// Fall back to HTTP when SSH is required but no username is available.
	if gerritInteractionType == "ssh" && gerritUsername == "" {
		gerritInteractionType = "http"
	}

	// Separate Gerrit name-based checkouts from arbitrary URL+Path checkouts.
	var extURLCheckouts, skinURLCheckouts []recipe.Checkout
	var extGerritNames, skinGerritNames []string
	for _, co := range spec.Code.Extensions {
		if co.URL != "" {
			extURLCheckouts = append(extURLCheckouts, co)
		} else {
			extGerritNames = append(extGerritNames, co.Name)
		}
	}
	for _, co := range spec.Code.Skins {
		if co.URL != "" {
			skinURLCheckouts = append(skinURLCheckouts, co)
		} else {
			skinGerritNames = append(skinGerritNames, co.Name)
		}
	}

	useShallow := spec.Source.Shallow
	if !useShallow {
		useShallow = configState.Effective.MwDev.Docker.ShallowClones
	}

	cloneOpts := mediawiki.CloneOpts{
		GetMediaWiki:          spec.Code.Core,
		GetGerritExtensions:   extGerritNames,
		GetGerritSkins:        skinGerritNames,
		UseShallow:            useShallow,
		UseGithub:             spec.Source.UseGithub,
		GerritInteractionType: gerritInteractionType,
		GerritUsername:        gerritUsername,
		DryRun:                dryRun,
	}

	if thisMW.MediaWikiIsPresent() {
		cloneOpts.GetMediaWiki = false
	}

	cloneOpts.GetGerritExtensions = filterMissingRepos(thisMW, "extensions", cloneOpts.GetGerritExtensions)
	cloneOpts.GetGerritSkins = filterMissingRepos(thisMW, "skins", cloneOpts.GetGerritSkins)

	if cloneOpts.AreThereThingsToClone() {
		if cloneOpts.GerritInteractionType == "ssh" && cloneOpts.GerritUsername == "" {
			return fmt.Errorf("gerrit username is required for ssh interaction type")
		}
		thisMW.CloneSetup(cloneOpts)
	}

	// Clone arbitrary URL+Path repos.
	allURLCheckouts := append(extURLCheckouts, skinURLCheckouts...)
	for _, co := range allURLCheckouts {
		destPath := co.Path
		if !filepath.IsAbs(destPath) {
			destPath = filepath.Join(thisMW.Path(""), destPath)
		}
		if _, err := os.Stat(destPath); err == nil {
			fmt.Printf("Skipping already-present repo at %s\n", co.Path)
			continue
		}
		fmt.Printf("Cloning %s into %s\n", co.URL, co.Path)
		if dryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("creating parent directory for %s: %w", co.Path, err)
		}
		cmd := osexec.Command("git", "clone", co.URL, destPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cloning %s: %w", co.URL, err)
		}
	}

	return nil
}

func checkoutNames(items []recipe.Checkout) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Name != "" {
			out = append(out, item.Name)
		}
	}
	return out
}

func filterMissingRepos(thisMW mediawiki.MediaWiki, kind string, names []string) []string {
	out := []string{}
	for _, n := range names {
		p := filepath.Clean(thisMW.Path(kind + "/" + n))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			out = append(out, n)
		}
	}
	return out
}

func servicesRequiredBySites(sites []recipe.Site) []string {
	required := []string{"mediawiki"}
	for _, site := range sites {
		switch site.DBType {
		case "mysql":
			required = append(required, "mysql")
		case "postgres":
			required = append(required, "postgres")
		}
	}
	return uniqueStrings(required)
}

func hasService(services []recipe.Service, name string) bool {
	for _, s := range services {
		if s.Name == name {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func applyServices(services []recipe.Service, m mwdd.MWDD, dryRun bool) error {
	for _, svc := range services {
		f := m.DockerCompose().File(svc.Name)
		if !f.Exists() {
			return fmt.Errorf("service file %s does not exist", f.String())
		}
		names := f.Contents().ServiceNames()
		if len(names) == 0 {
			return fmt.Errorf("service file %s does not define any services", f.String())
		}

		switch svc.State {
		case "started":
			if dryRun {
				fmt.Printf("[dry-run] docker compose up -d for service file %s (%v)\n", svc.Name, names)
				continue
			}
			if err := m.DockerCompose().Up(names, dockercompose.UpOptions{Detached: true}); err != nil {
				return err
			}
		case "stopped":
			if dryRun {
				fmt.Printf("[dry-run] docker compose stop for service file %s (%v)\n", svc.Name, names)
				continue
			}
			if err := m.DockerCompose().Stop(names); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyJobRunnerSites(m mwdd.MWDD, sites []string, dryRun bool) error {
	if len(sites) == 0 {
		return nil
	}
	jobRunnerSitesFile := filepath.Clean(filepath.Join(m.Directory(), "mediawiki", "jobrunner-sites"))
	for _, site := range uniqueStrings(nonEmptyStrings(sites)) {
		if dryRun {
			fmt.Printf("[dry-run] add jobrunner site %s to %s\n", site, jobRunnerSitesFile)
			continue
		}
		filesutil.AddLineUnique(site, jobRunnerSitesFile)
	}
	return nil
}

func installSite(m mwdd.MWDD, thisMW mediawiki.MediaWiki, site recipe.Site, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] install wiki site dbname=%s dbtype=%s\n", site.DBName, site.DBType)
		return nil
	}

	if err := ensureLocalSettingsBase(thisMW); err != nil {
		return err
	}

	if !thisMW.LocalSettingsContains("/mwdd/MwddSettings.php") {
		return fmt.Errorf("LocalSettings.php is missing /mwdd/MwddSettings.php include")
	}

	if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
		User:           "root",
		CommandAndArgs: []string{"mkdir", "-p", "/var/www/html/w/cache/docker/" + site.DBName},
	}); err != nil {
		return err
	}

	if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
		User:           "root",
		CommandAndArgs: []string{"chown", "-R", "nobody", "/var/www/html/w/cache", "/var/www/html/w/images", "/var/log/mediawiki"},
	}); err != nil {
		return err
	}

	if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
		User:           "root",
		CommandAndArgs: []string{"chmod", "-R", "0777", "/var/www/html/w/cache", "/var/www/html/w/images"},
	}); err != nil {
		return err
	}

	domain := site.DBName + ".mediawiki.local.wmftest.net"
	m.RecordHostUsageBySite(domain)

	serverLink := "http://" + domain + ":" + m.Env().Get("PORT")
	backupSuffix := time.Now().Format("20060102150405")
	backupPath := "/var/www/html/w/LocalSettings.php.recipe.bak." + backupSuffix
	restoredLocalSettings := false
	restoreLocalSettings := func() {
		if restoredLocalSettings {
			return
		}
		_ = m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
			User:           "root",
			CommandAndArgs: []string{"mv", backupPath, "/var/www/html/w/LocalSettings.php"},
		})
		restoredLocalSettings = true
	}

	if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
		User:           "root",
		CommandAndArgs: []string{"mv", "/var/www/html/w/LocalSettings.php", backupPath},
	}); err != nil {
		return err
	}

	defer restoreLocalSettings()

	if site.DBType == "mysql" {
		if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
			User:           "nobody",
			CommandAndArgs: []string{"/wait-for-it.sh", "mysql:3306"},
		}); err != nil {
			return err
		}
	}
	if site.DBType == "postgres" {
		if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
			User:           "nobody",
			CommandAndArgs: []string{"/wait-for-it.sh", "postgres:5432"},
		}); err != nil {
			return err
		}
	}

	installArgs := []string{
		"php", "/mwdd/MwddInstall.php",
		"--confpath", "/tmp",
		"--server", serverLink,
		"--dbtype", site.DBType,
		"--dbname", site.DBName,
		"--lang", "en",
		"--pass", "mwddpassword",
		"docker-" + site.DBName,
		"admin",
	}

	if site.DBType == "sqlite" {
		installArgs = slices.Insert(installArgs, 8, "--dbpath", "/var/www/html/w/cache/docker")
	} else {
		installArgs = slices.Insert(installArgs, 8,
			"--dbuser", "root",
			"--dbpass", "toor",
			"--dbserver", site.DBType,
		)
	}

	if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
		User:           "nobody",
		CommandAndArgs: installArgs,
	}); err != nil {
		return err
	}

	// Move LocalSettings back before running update, as update needs /var/www/html/w/LocalSettings.php.
	restoreLocalSettings()

	if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
		User:           "nobody",
		CommandAndArgs: []string{"php", "/var/www/html/w/maintenance/update.php", "--wiki", site.DBName, "--quick"},
	}); err != nil {
		return err
	}

	fmt.Printf("Installed wiki %s (%s)\n", site.DBName, site.DBType)
	return nil
}

func ensureLocalSettingsBase(thisMW mediawiki.MediaWiki) error {
	if !thisMW.LocalSettingsIsPresent() {
		base := "<?php\nrequire_once '/mwdd/MwddSettings.php';\n"
		if thisMW.VectorIsPresent() {
			base += "\nwfLoadSkin('Vector');\n"
		}
		return os.WriteFile(thisMW.LocalSettingsPath(), []byte(base), 0o644)
	}

	contentBytes, err := os.ReadFile(thisMW.LocalSettingsPath())
	if err != nil {
		return err
	}
	content := string(contentBytes)
	if !strings.Contains(content, "/mwdd/MwddSettings.php") {
		content += "\nrequire_once '/mwdd/MwddSettings.php';\n"
		if err := os.WriteFile(thisMW.LocalSettingsPath(), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func applyLocalSettings(thisMW mediawiki.MediaWiki, recipeName string, ls recipe.LocalSettings, dryRun bool) error {
	hasFiles := len(ls.Files.Shared) > 0 || len(ls.Files.PerWiki) > 0
	hasAppend := strings.TrimSpace(ls.AppendPHP) != ""
	hasYAML := strings.TrimSpace(ls.YAMLSettingsFile) != ""

	if !hasFiles && !hasAppend && !hasYAML {
		return nil
	}

	if dryRun {
		fmt.Println("[dry-run] apply LocalSettings")
		return nil
	}

	if err := ensureLocalSettingsBase(thisMW); err != nil {
		return err
	}

	// Prefer LocalSettings.d files over appending to LocalSettings.php.
	if hasFiles {
		return applyLocalSettingsFiles(thisMW, recipeName, ls.Files)
	}

	// Legacy: append to LocalSettings.php directly.
	contentBytes, err := os.ReadFile(thisMW.LocalSettingsPath())
	if err != nil {
		return err
	}
	content := string(contentBytes)

	if hasAppend {
		appendPHP := strings.TrimSpace(ls.AppendPHP)
		content = removeManagedRecipeLocalSettingsBlocks(content)
		content = strings.ReplaceAll(content, "\n"+appendPHP+"\n", "\n")
		content = strings.ReplaceAll(content, appendPHP+"\n", "")
		content = strings.ReplaceAll(content, "\n"+appendPHP, "")

		managedBlock := strings.TrimSpace(strings.Join([]string{
			"// BEGIN MWCLI RECIPE: " + recipeName,
			appendPHP,
			"// END MWCLI RECIPE: " + recipeName,
		}, "\n"))
		content += "\n" + managedBlock + "\n"
	}

	if hasYAML {
		settingsPath := filepath.Clean(filepath.Join(thisMW.Directory(), ls.YAMLSettingsFile))
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(settingsPath, []byte(strings.TrimSpace(ls.YAMLSettings)+"\n"), 0o644); err != nil {
			return err
		}

		loadStmt := "$wgSettings->loadFile( '" + strings.TrimSpace(ls.YAMLSettingsFile) + "' );"
		if !strings.Contains(content, loadStmt) {
			content += "\n" + loadStmt + "\n"
		}
	}

	return os.WriteFile(thisMW.LocalSettingsPath(), []byte(content), 0o644)
}

const recipeLocalSettingsMarker = "// Generated by mwcli recipe"

// applyLocalSettingsFiles writes recipe-managed PHP files into LocalSettings.d/
// so they are loaded by MwddSettings.php automatically.
func applyLocalSettingsFiles(thisMW mediawiki.MediaWiki, recipeName string, files recipe.LocalSettingsFiles) error {
	localSettingsDPath := filepath.Clean(filepath.Join(thisMW.Path(""), "LocalSettings.d"))

	// Write shared files (loaded for all wikis).
	for i, f := range files.Shared {
		fileName := "recipe-" + recipeName + ".php"
		if i > 0 {
			fileName = "recipe-" + recipeName + "-" + strconv.Itoa(i) + ".php"
		}
		filePath := filepath.Clean(filepath.Join(localSettingsDPath, fileName))
		if err := os.MkdirAll(localSettingsDPath, 0o755); err != nil {
			return err
		}
		content := "<?php\n" + recipeLocalSettingsMarker + " — " + recipeName + "\n" + strings.TrimSpace(f.Content) + "\n"
		fmt.Printf("Writing %s\n", filePath)
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			return err
		}
	}

	// Write per-wiki files (loaded only for that wiki's $wgDBname).
	for wiki, wikiFiles := range files.PerWiki {
		for i, f := range wikiFiles {
			fileName := "recipe-" + recipeName + ".php"
			if i > 0 {
				fileName = "recipe-" + recipeName + "-" + strconv.Itoa(i) + ".php"
			}
			dirPath := filepath.Clean(filepath.Join(localSettingsDPath, wiki))
			filePath := filepath.Clean(filepath.Join(dirPath, fileName))
			if err := os.MkdirAll(dirPath, 0o755); err != nil {
				return err
			}
			content := "<?php\n" + recipeLocalSettingsMarker + " — " + recipeName + "\n" + strings.TrimSpace(f.Content) + "\n"
			fmt.Printf("Writing %s\n", filePath)
			if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
				return err
			}
		}
	}

	return nil
}

func removeManagedRecipeLocalSettings(thisMW mediawiki.MediaWiki, dryRun bool) error {
	if dryRun {
		fmt.Println("[dry-run] remove managed mwcli recipe settings")
		return nil
	}

	// Clean legacy blocks from LocalSettings.php.
	if thisMW.LocalSettingsIsPresent() {
		contentBytes, err := os.ReadFile(thisMW.LocalSettingsPath())
		if err != nil {
			return err
		}
		content := removeManagedRecipeLocalSettingsBlocks(string(contentBytes))
		if err := os.WriteFile(thisMW.LocalSettingsPath(), []byte(content), 0o644); err != nil {
			return err
		}
	}

	// Clean LocalSettings.d/ recipe files.
	localSettingsDPath := filepath.Clean(filepath.Join(thisMW.Path(""), "LocalSettings.d"))
	if err := cleanupRecipeLocalSettingsDFiles(localSettingsDPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// cleanupRecipeLocalSettingsDFiles removes recipe-managed PHP files from a
// LocalSettings.d directory (and per-wiki subdirectories).
func cleanupRecipeLocalSettingsDFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		entryPath := filepath.Clean(filepath.Join(dir, entry.Name()))
		if entry.IsDir() {
			if err := cleanupRecipeLocalSettingsDFiles(entryPath); err != nil {
				return err
			}
			remaining, _ := os.ReadDir(entryPath)
			if len(remaining) == 0 {
				_ = os.Remove(entryPath)
			}
			continue
		}
		if filepath.Ext(entry.Name()) != ".php" {
			continue
		}
		contentBytes, err := os.ReadFile(entryPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if !strings.Contains(string(contentBytes), recipeLocalSettingsMarker) {
			continue
		}
		if err := os.Remove(entryPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func removeManagedRecipeLocalSettingsBlocks(content string) string {
	start := "// BEGIN MWCLI RECIPE:"
	end := "// END MWCLI RECIPE:"

	for {
		startIdx := strings.Index(content, start)
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(content[startIdx:], end)
		if endIdx == -1 {
			break
		}
		endLineIdx := strings.Index(content[startIdx+endIdx:], "\n")
		if endLineIdx == -1 {
			content = content[:startIdx]
			break
		}
		content = content[:startIdx] + content[startIdx+endIdx+endLineIdx+1:]
	}

	return content
}

func syncComposerLocalAndUpdate(m mwdd.MWDD, thisMW mediawiki.MediaWiki, code recipe.Code, dryRun bool) error {
	includes := composerLocalIncludesForCode(thisMW, code)
	if len(includes) == 0 {
		return nil
	}

	composerLocalPath := filepath.Clean(filepath.Join(thisMW.Path(""), "composer.local.json"))
	composerLocal, err := readComposerLocalJSON(composerLocalPath)
	if err != nil {
		return err
	}

	updatedIncludes, changed := setComposerMergeIncludes(composerLocal, includes)
	if dryRun {
		if changed {
			fmt.Printf("[dry-run] update %s includes: %v\n", composerLocalPath, updatedIncludes)
		}
		fmt.Println("[dry-run] composer update --with-all-dependencies")
		return nil
	}

	if changed {
		if err := writeComposerLocalJSON(composerLocalPath, composerLocal); err != nil {
			return err
		}
	}

	composerCmd := strings.Join([]string{
		"set -e",
		"mkdir -p /tmp/composer-home /tmp/composer-cache",
		"COMPOSER_HOME=/tmp/composer-home COMPOSER_CACHE_DIR=/tmp/composer-cache " +
			"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=safe.directory GIT_CONFIG_VALUE_0=/var/www/html/w " +
			"composer update --with-all-dependencies",
	}, " && ")

	if err := m.DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
		User:           docker.CurrentUserAndGroupForDockerExecution(),
		CommandAndArgs: []string{"sh", "-lc", composerCmd},
	}); err != nil {
		return fmt.Errorf("composer update (with composer.local.json includes) failed: %w", err)
	}

	return nil
}

func composerLocalIncludesForCode(thisMW mediawiki.MediaWiki, code recipe.Code) []string {
	includes := []string{}
	seen := map[string]bool{}

	add := func(kind string, co recipe.Checkout) {
		relativeRepoPath := strings.TrimSpace(co.Path)
		if relativeRepoPath == "" {
			if co.Name == "" {
				return
			}
			relativeRepoPath = filepath.ToSlash(filepath.Join(kind, co.Name))
		}
		if filepath.IsAbs(relativeRepoPath) {
			return
		}

		relativeComposerPath := filepath.ToSlash(filepath.Join(relativeRepoPath, "composer.json"))
		hostComposerPath := filepath.Clean(filepath.Join(thisMW.Path(""), relativeComposerPath))
		if _, err := os.Stat(hostComposerPath); err != nil {
			return
		}
		if !seen[relativeComposerPath] {
			seen[relativeComposerPath] = true
			includes = append(includes, relativeComposerPath)
		}
	}

	for _, ext := range code.Extensions {
		add("extensions", ext)
	}
	for _, skin := range code.Skins {
		add("skins", skin)
	}

	sort.Strings(includes)
	return includes
}

func readComposerLocalJSON(path string) (map[string]interface{}, error) {
	root := map[string]interface{}{}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return root, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return root, nil
	}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return root, nil
}

func writeComposerLocalJSON(path string, root map[string]interface{}) error {
	b, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func setComposerMergeIncludes(root map[string]interface{}, includes []string) ([]string, bool) {
	if root == nil {
		root = map[string]interface{}{}
	}

	extra := mapStringAny(root["extra"])
	mergePlugin := mapStringAny(extra["merge-plugin"])

	existing := anyToStringSlice(mergePlugin["include"])
	all := uniqueStrings(append(existing, includes...))
	sort.Strings(all)

	changed := !stringSlicesEqual(existing, all)
	if changed {
		mergePlugin["include"] = all
		extra["merge-plugin"] = mergePlugin
		root["extra"] = extra
	}

	return all, changed
}

func mapStringAny(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func anyToStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}

	out := []string{}
	switch t := v.(type) {
	case []string:
		out = append(out, t...)
	case []interface{}:
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
	}
	return uniqueStrings(out)
}

func stringSlicesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func runMaintenanceSteps(m mwdd.MWDD, steps []recipe.ContainerCommandStep, dryRun bool) error {
	if len(steps) == 0 {
		return nil
	}

	containerID, err := m.DockerCompose().ContainerID("mediawiki")
	if err != nil {
		return err
	}

	for i, step := range steps {
		name := step.Name
		if name == "" {
			name = "step-" + strconv.Itoa(i+1)
		}
		if dryRun {
			fmt.Printf("[dry-run] maintenance %s: %v\n", name, step.Command)
			continue
		}

		env := mapToEnv(step.Env)
		exitCode := docker.Exec(containerID, docker.ExecOptions{
			Command:    step.Command,
			Env:        env,
			User:       defaultUser(step.User),
			WorkingDir: defaultWorkingDir(step.WorkingDir),
		})
		if exitCode != 0 {
			return fmt.Errorf("maintenance step %q failed with exit code %d", name, exitCode)
		}
	}

	return nil
}

func defaultUser(user string) string {
	if user != "" {
		return user
	}
	return docker.CurrentUserAndGroupForDockerExecution()
}

func defaultWorkingDir(workingDir string) string {
	if workingDir != "" {
		return workingDir
	}
	return "/var/www/html/w"
}

func mapToEnv(envMap map[string]string) []string {
	out := make([]string, 0, len(envMap))
	for k, v := range envMap {
		out = append(out, k+"="+v)
	}
	return out
}

func applyPatches(mediaWikiPath string, patches []recipe.Patch, dryRun bool) error {
	for i, patch := range patches {
		name := patch.Name
		if name == "" {
			name = "patch-" + strconv.Itoa(i+1)
		}

		repoPath := patch.RepoPath
		if !filepath.IsAbs(repoPath) {
			repoPath = filepath.Clean(filepath.Join(mediaWikiPath, repoPath))
		}
		cherryPickRef := patch.CherryPick
		if cherryPickRef == "" {
			cherryPickRef = "FETCH_HEAD"
		}

		if dryRun {
			fmt.Printf("[dry-run] patch %s: git -C %s fetch %v && cherry-pick %s\n", name, repoPath, patch.Fetch, cherryPickRef)
			continue
		}

		if err := runGit(repoPath, append([]string{"fetch"}, patch.Fetch...)...); err != nil {
			return fmt.Errorf("patch %q fetch failed: %w", name, err)
		}
		if err := runGit(repoPath, "cherry-pick", cherryPickRef); err != nil {
			return fmt.Errorf("patch %q cherry-pick failed: %w", name, err)
		}
	}
	return nil
}

// waitForSites polls each site's API endpoint until it responds, or times out.
func waitForSites(sites []recipe.Site, port string, dryRun bool) error {
	if len(sites) == 0 {
		return nil
	}
	if dryRun {
		fmt.Println("[dry-run] wait for sites to respond")
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, site := range sites {
		apiURL := "http://" + site.DBName + ".mediawiki.local.wmftest.net:" + port + "/w/api.php?action=query&format=json"
		fmt.Printf("Waiting for %s site to respond...\n", site.DBName)
		ready := false
		for i := 0; i < 30; i++ {
			resp, err := client.Get(apiURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					ready = true
					break
				}
			}
			time.Sleep(2 * time.Second)
		}
		if !ready {
			return fmt.Errorf("timed out waiting for %s site to respond", site.DBName)
		}
		fmt.Printf("%s site is ready.\n", site.DBName)
	}
	return nil
}

func applyContent(m mwdd.MWDD, sites []recipe.Site, content recipe.Content, dryRun bool) error {
	if len(content.Wikibase.Properties) == 0 && len(content.Wikibase.Items) == 0 && len(content.Pages) == 0 {
		return nil
	}

	port := m.Env().Get("PORT")
	if err := waitForSites(sites, port, dryRun); err != nil {
		return err
	}

	// Find the repo wiki (default site) for wikibase operations
	repoSite := ""
	for _, site := range sites {
		if site.DBName == "default" {
			repoSite = site.DBName
			break
		}
	}
	if repoSite == "" && len(sites) > 0 {
		repoSite = sites[0].DBName
	}

	// Content creation uses anonymous edits (no login required).
	// The mediawiki content functions fall back to the anonymous CSRF token
	// when username and password are empty.
	username := ""
	password := ""

	// Create wikibase properties
	for _, prop := range content.Wikibase.Properties {
		if dryRun {
			fmt.Printf("[dry-run] create wikibase property %s: %s (%s)\n", prop.ID, prop.Label, prop.Datatype)
			continue
		}

		wikiURL := "http://" + repoSite + ".mediawiki.local.wmftest.net:" + port + "/w/api.php"
		fmt.Printf("Creating wikibase property %s on %s...\n", prop.ID, repoSite)

		propInput := mediawiki.WikibasePropertyInput{
			ID:       prop.ID,
			Label:    prop.Label,
			Datatype: prop.Datatype,
		}

		if err := mediawiki.CreateWikibaseProperty(wikiURL, username, password, propInput); err != nil {
			return fmt.Errorf("failed to create wikibase property %s: %w", prop.ID, err)
		}
	}

	// Create wikibase items
	for _, item := range content.Wikibase.Items {
		if dryRun {
			fmt.Printf("[dry-run] create wikibase item %s: %s\n", item.ID, item.Label)
			continue
		}

		wikiURL := "http://" + repoSite + ".mediawiki.local.wmftest.net:" + port + "/w/api.php"
		fmt.Printf("Creating wikibase item %s on %s...\n", item.ID, repoSite)

		claims := []mediawiki.WikibaseItemClaimInput{}
		for _, claim := range item.Claims {
			claims = append(claims, mediawiki.WikibaseItemClaimInput{
				Property: claim.Property,
				Value:    claim.Value,
			})
		}

		itemInput := mediawiki.WikibaseItemInput{
			ID:     item.ID,
			Label:  item.Label,
			Claims: claims,
		}

		if err := mediawiki.CreateWikibaseItem(wikiURL, username, password, itemInput); err != nil {
			return fmt.Errorf("failed to create wikibase item %s: %w", item.ID, err)
		}
	}

	// Create pages
	for _, page := range content.Pages {
		if dryRun {
			fmt.Printf("[dry-run] create page %s on %s: %s\n", page.Title, page.Wiki, page.Text)
			continue
		}

		wikiURL := "http://" + page.Wiki + ".mediawiki.local.wmftest.net:" + port + "/w/api.php"
		fmt.Printf("Creating page %s on %s...\n", page.Title, page.Wiki)

		pageInput := mediawiki.PageInput{
			Title:   page.Title,
			Text:    page.Text,
			Summary: "Created by recipe",
		}

		if err := mediawiki.CreatePage(wikiURL, username, password, pageInput); err != nil {
			return fmt.Errorf("failed to create page %s on %s: %w", page.Title, page.Wiki, err)
		}
	}

	return nil
}

func runGit(repoPath string, args ...string) error {
	cmd := osexec.Command("git", append([]string{"-C", repoPath}, args...)...) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
