package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/daFish/gogo-meta/internal/ssh"
	"github.com/spf13/cobra"
)

var repoNamePattern = regexp.MustCompile(`/([^/]+?)(\.git)?$`)

func extractRepoName(url string) string {
	matches := repoNamePattern.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return "repo"
}

func newGitCloneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <url>",
		Short: "Clone a meta repository and all child repositories",
		Args:  cobra.ExactArgs(1),
		RunE:  runGitClone,
	}
	cmd.Flags().StringP("directory", "d", "", "Target directory name")
	return cmd
}

func runGitClone(cmd *cobra.Command, args []string) error {
	url := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	directory, _ := cmd.Flags().GetString("directory")
	repoName := directory
	if repoName == "" {
		repoName = extractRepoName(url)
	}

	targetDir := filepath.Join(cwd, repoName)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("directory %q already exists", repoName)
	}

	ssh.EnsureSSHHostsKnown([]string{url})

	output.Info(fmt.Sprintf("Cloning meta repository: %s", url))

	exec := executor.NewShellExecutor()
	ctx := context.Background()

	cloneResult, err := exec.Execute(ctx, fmt.Sprintf(`git clone "%s" "%s"`, url, repoName), executor.Options{Cwd: cwd})
	if err != nil {
		return err
	}

	if cloneResult.ExitCode != 0 {
		output.Error("Failed to clone meta repository")
		output.CommandOutput(cloneResult.Stdout, cloneResult.Stderr)
		os.Exit(1)
		return nil
	}

	output.Success(fmt.Sprintf("Cloned meta repository to %s", repoName))

	metaPath, err := config.FindMetaFileUp(targetDir)
	if err != nil || metaPath == "" {
		output.Warning("No config file found in cloned repository")
		return nil
	}

	configResult, err := config.ReadMetaConfig(targetDir, []string{})
	if err != nil {
		return err
	}

	projects := configResult.Config.Projects
	if len(projects) == 0 {
		output.Info("No child repositories defined in .gogo")
		return nil
	}

	urls := make([]string, 0, len(projects))
	for _, u := range projects {
		urls = append(urls, u)
	}
	_, failedHosts := ssh.EnsureSSHHostsKnown(urls)

	if len(failedHosts) > 0 {
		output.Warning(fmt.Sprintf("Could not verify SSH host keys for: %s. Clone may fail.", joinStrings(failedHosts)))
	}

	output.Info(fmt.Sprintf("Cloning %d child repositories...", len(projects)))

	successCount := 0
	failCount := 0

	for projectPath, projectURL := range projects {
		projectDir := filepath.Join(targetDir, projectPath)

		if _, err := os.Stat(projectDir); err == nil {
			output.ProjectStatus(projectPath, "success", "already exists")
			successCount++
			continue
		}

		parentDir := filepath.Dir(projectDir)
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			output.ProjectStatus(projectPath, "error", err.Error())
			failCount++
			continue
		}

		result, err := exec.Execute(ctx, fmt.Sprintf(`git clone "%s" "%s"`, projectURL, filepath.Base(projectPath)), executor.Options{Cwd: parentDir})
		if err != nil {
			output.ProjectStatus(projectPath, "error", err.Error())
			failCount++
			continue
		}

		if result.ExitCode == 0 {
			output.ProjectStatus(projectPath, "success", "cloned")
			successCount++
		} else {
			msg := result.Stderr
			if msg == "" {
				msg = "clone failed"
			}
			output.ProjectStatus(projectPath, "error", msg)
			failCount++
		}
	}

	output.Summary(output.SummaryData{
		Success: successCount,
		Failed:  failCount,
		Total:   len(projects),
	})

	if failCount > 0 {
		os.Exit(1)
	}
	return nil
}

func joinStrings(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += v
	}
	return result
}
