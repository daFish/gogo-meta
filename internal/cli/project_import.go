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

func newProjectImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <folder> [url]",
		Short: "Import an existing project",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runProjectImport,
	}
	cmd.Flags().Bool("no-clone", false, "Register project without cloning")
	return cmd
}

func runProjectImport(cmd *cobra.Command, args []string) error {
	folder := args[0]
	var url string
	if len(args) > 1 {
		url = args[1]
	}

	metaDir, err := requireMetaDir()
	if err != nil {
		return err
	}

	projectDir := filepath.Join(metaDir, folder)
	noClone, _ := cmd.Flags().GetBool("no-clone")

	exec := executor.NewShellExecutor()
	ctx := context.Background()

	if _, err := os.Stat(projectDir); err == nil {
		// Project directory exists.
		existingURL := ""
		result, err := exec.Execute(ctx, "git remote get-url origin", executor.Options{Cwd: projectDir})
		if err == nil && result.ExitCode == 0 && result.Stdout != "" {
			existingURL = result.Stdout
		}

		if existingURL == "" && url == "" {
			return fmt.Errorf("directory %q exists but has no remote. Provide a URL to set one", folder)
		}

		finalURL := url
		if finalURL == "" {
			finalURL = existingURL
		}

		if url != "" && existingURL != "" && url != existingURL {
			output.Warning("Existing remote URL differs from provided URL")
			output.Info(fmt.Sprintf("Existing: %s", existingURL))
			output.Info(fmt.Sprintf("Provided: %s", url))
		}

		configResult, err := config.ReadMetaConfig(metaDir, []string{})
		if err != nil {
			return err
		}

		updatedConfig := config.AddProject(configResult.Config, folder, finalURL)
		if err := config.WriteMetaConfig(metaDir, updatedConfig, configResult.Format); err != nil {
			return err
		}

		output.Success(fmt.Sprintf("Imported existing project %q", folder))
		output.Info(fmt.Sprintf("Repository URL: %s", finalURL))
	} else {
		// Project directory doesn't exist.
		if url == "" {
			return fmt.Errorf("URL is required when importing a non-existent project")
		}

		if noClone {
			configResult, err := config.ReadMetaConfig(metaDir, []string{})
			if err != nil {
				return err
			}
			updatedConfig := config.AddProject(configResult.Config, folder, url)
			if err := config.WriteMetaConfig(metaDir, updatedConfig, configResult.Format); err != nil {
				return err
			}
			added, _ := config.AddToGitignore(metaDir, folder)
			output.Success(fmt.Sprintf("Registered project %q (not cloned)", folder))
			if added {
				output.Info(fmt.Sprintf("Added %s to .gitignore", folder))
			}
			output.Info("Run \"gogo git update\" to clone missing projects")
			return nil
		}

		output.Info(fmt.Sprintf("Cloning %s into %s...", url, folder))

		parentDir := filepath.Dir(projectDir)
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return err
		}

		cloneResult, err := exec.Execute(ctx, fmt.Sprintf(`git clone "%s" "%s"`, url, filepath.Base(folder)), executor.Options{Cwd: parentDir})
		if err != nil {
			return err
		}
		if cloneResult.ExitCode != 0 {
			return fmt.Errorf("failed to clone repository: %s", cloneResult.Stderr)
		}

		configResult, err := config.ReadMetaConfig(metaDir, []string{})
		if err != nil {
			return err
		}
		updatedConfig := config.AddProject(configResult.Config, folder, url)
		if err := config.WriteMetaConfig(metaDir, updatedConfig, configResult.Format); err != nil {
			return err
		}

		output.Success(fmt.Sprintf("Imported project %q", folder))
	}

	added, _ := config.AddToGitignore(metaDir, folder)
	if added {
		output.Info(fmt.Sprintf("Added %s to .gitignore", folder))
	}

	return nil
}
