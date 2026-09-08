package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func newGitUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Clone missing repositories",
		Args:  cobra.NoArgs,
		RunE:  runGitUpdate,
	}
	addFilterFlags(cmd)
	addParallelFlags(cmd)
	return cmd
}

func runGitUpdate(cmd *cobra.Command, _ []string) error {
	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}

	configResult, err := resolveConfig()
	if err != nil {
		return err
	}

	// Keep the gogo-meta-managed block in .git/info/exclude in sync with the full
	// .gogo.local project set. Filter-independent: dropping a project from
	// .gogo.local must remove its stale entry too.
	if err := syncLocalExcludes(metaDir, configResult.LocalProjects); err != nil {
		return err
	}

	filterOpts, err := resolveFilterOptionsWithConfig(cmd, &configResult.Config)
	if err != nil {
		return err
	}

	// Get filtered project entries.
	projectPaths := make([]string, 0, len(configResult.Config.Projects))
	for k := range configResult.Config.Projects {
		projectPaths = append(projectPaths, k)
	}
	filteredPaths := filter.Apply(projectPaths, filterOpts)

	if len(filteredPaths) == 0 {
		output.Warning("No projects match the specified filters")
		return nil
	}

	output.Info(fmt.Sprintf("Checking %d repositories...", len(filteredPaths)))

	type missing struct {
		path string
		url  string
	}
	var missingRepos []missing

	for _, projectPath := range filteredPaths {
		projectDir := filepath.Join(metaDir, projectPath)
		if _, err := os.Stat(projectDir); os.IsNotExist(err) {
			url := configResult.Config.Projects[projectPath]
			missingRepos = append(missingRepos, missing{path: projectPath, url: url})
		}
	}

	if len(missingRepos) == 0 {
		output.Success("All repositories are already cloned")
		return nil
	}

	urls := make([]string, len(missingRepos))
	for i, m := range missingRepos {
		urls[i] = m.url
	}
	warnUnverifiedSSHHosts(urls)

	output.Info(fmt.Sprintf("Cloning %d missing repositories...", len(missingRepos)))

	targets := make([]cloneTarget, len(missingRepos))
	for i, m := range missingRepos {
		targets[i] = cloneTarget{Path: m.path, URL: m.url}
	}

	outcomes := cloneMissing(cmd.Context(), executor.NewShellExecutor(), metaDir, targets,
		getBoolFlag(cmd, "parallel"), getIntFlag(cmd, "concurrency"))

	successCount := 0
	failCount := 0
	for _, o := range outcomes {
		if o.Err == "" {
			output.ProjectStatus(o.Path, "success", "cloned")
			successCount++
			continue
		}
		output.ProjectStatus(o.Path, "error", o.Err)
		failCount++
	}

	output.Summary(output.SummaryData{
		Success: successCount,
		Failed:  failCount,
		Total:   len(missingRepos),
	})

	if failCount > 0 {
		os.Exit(1)
	}
	return nil
}
