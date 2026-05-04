package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func newProjectCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <folder> <url>",
		Short: "Create a new project",
		Args:  cobra.ExactArgs(2),
		RunE:  runProjectCreate,
	}
}

func runProjectCreate(_ *cobra.Command, args []string) error {
	folder := args[0]
	url := args[1]

	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}

	projectDir := filepath.Join(metaDir, folder)
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("directory %q already exists", folder)
	}

	output.Info(fmt.Sprintf("Creating new project: %s", folder))

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}

	exec := executor.NewShellExecutor()
	ctx := context.Background()

	initResult, err := exec.Execute(ctx, "git init", executor.Options{Cwd: projectDir})
	if err != nil {
		return err
	}
	if initResult.ExitCode != 0 {
		return fmt.Errorf("failed to initialize git repository: %s", initResult.Stderr)
	}

	remoteResult, err := exec.Execute(ctx, fmt.Sprintf(`git remote add origin "%s"`, url), executor.Options{Cwd: projectDir})
	if err != nil {
		return err
	}
	if remoteResult.ExitCode != 0 {
		return fmt.Errorf("failed to add remote: %s", remoteResult.Stderr)
	}

	configResult, err := config.ReadMetaConfig(metaDir, []string{})
	if err != nil {
		return err
	}

	updatedConfig := config.AddProject(configResult.Config, folder, url)
	if err := config.WriteMetaConfig(metaDir, updatedConfig, configResult.Format); err != nil {
		return err
	}

	gitignorePath := filepath.Join(metaDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.WriteString("\n" + folder + "\n")
			_ = f.Close()
			output.Info(fmt.Sprintf("Added %s to .gitignore", folder))
		}
	}

	output.Success(fmt.Sprintf("Created project %q", folder))
	output.Info(fmt.Sprintf("Repository initialized with remote: %s", url))
	return nil
}
