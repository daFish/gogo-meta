package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/daFish/gogo-meta/internal/ssh"
	"github.com/spf13/cobra"
)

func newGitUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Clone missing repositories",
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

	filterOpts, err := resolveFilterOptions(cmd)
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
	_, failedHosts := ssh.EnsureSSHHostsKnown(urls)

	if len(failedHosts) > 0 {
		output.Warning(fmt.Sprintf("Could not verify SSH host keys for: %s. Clone may fail.", joinStrings(failedHosts)))
	}

	output.Info(fmt.Sprintf("Cloning %d missing repositories...", len(missingRepos)))

	exec := executor.NewShellExecutor()
	ctx := context.Background()

	successCount := 0
	failCount := 0

	for _, m := range missingRepos {
		projectDir := filepath.Join(metaDir, m.path)
		parentDir := filepath.Dir(projectDir)

		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			output.ProjectStatus(m.path, "error", err.Error())
			failCount++
			continue
		}

		result, err := exec.Execute(ctx, fmt.Sprintf(`git clone "%s" "%s"`, m.url, filepath.Base(m.path)), executor.Options{Cwd: parentDir})
		if err != nil {
			output.ProjectStatus(m.path, "error", err.Error())
			failCount++
			continue
		}

		if result.ExitCode == 0 {
			output.ProjectStatus(m.path, "success", "cloned")
			successCount++
		} else {
			msg := result.Stderr
			if msg == "" {
				msg = "clone failed"
			}
			output.ProjectStatus(m.path, "error", msg)
			failCount++
		}
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
