package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/filter"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

type packageJSON struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func newNpmLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link packages across repositories",
		RunE:  runNpmLink,
	}
	cmd.Flags().Bool("all", false, "Link all projects bidirectionally")
	addFilterFlags(cmd)
	return cmd
}

func runNpmLink(cmd *cobra.Command, _ []string) error {
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

	projectPaths := config.GetProjectPaths(configResult.Config)
	projectPaths = filter.Apply(projectPaths, filterOpts)

	if len(projectPaths) == 0 {
		output.Warning("No projects match the specified filters")
		return nil
	}

	type projectInfo struct {
		path    string
		pkgJSON packageJSON
	}

	projectPackages := make(map[string]projectInfo)
	for _, projectPath := range projectPaths {
		fullPath := filepath.Join(metaDir, projectPath)
		pkg, err := readPackageJSON(fullPath)
		if err != nil || pkg == nil || pkg.Name == "" {
			continue
		}
		projectPackages[pkg.Name] = projectInfo{path: fullPath, pkgJSON: *pkg}
	}

	if len(projectPackages) == 0 {
		output.Warning("No projects with package.json found")
		return nil
	}

	output.Info(fmt.Sprintf("Found %d linkable projects", len(projectPackages)))

	allFlag, _ := cmd.Flags().GetBool("all")
	linkCount := 0

	exec := executor.NewShellExecutor()
	ctx := context.Background()

	if allFlag {
		for consumerName, consumer := range projectPackages {
			allDeps := make(map[string]string)
			for k, v := range consumer.pkgJSON.Dependencies {
				allDeps[k] = v
			}
			for k, v := range consumer.pkgJSON.DevDependencies {
				allDeps[k] = v
			}

			for depName := range allDeps {
				provider, ok := projectPackages[depName]
				if !ok {
					continue
				}

				nodeModulesPath := filepath.Join(consumer.path, "node_modules", depName)

				// Remove existing link/dir.
				_ = os.RemoveAll(nodeModulesPath)

				parentDir := filepath.Dir(nodeModulesPath)
				if err := os.MkdirAll(parentDir, 0o755); err != nil {
					output.Error(fmt.Sprintf("Failed to create symlink: %s -> %s", nodeModulesPath, provider.path))
					continue
				}

				if err := os.Symlink(provider.path, nodeModulesPath); err != nil {
					output.Error(fmt.Sprintf("Failed to create symlink: %s -> %s", nodeModulesPath, provider.path))
					continue
				}

				output.ProjectStatus(consumerName, "success", fmt.Sprintf("linked %s", depName))
				linkCount++
			}
		}
	} else {
		for pkgName, info := range projectPackages {
			output.Info(fmt.Sprintf("Creating global link for %s...", pkgName))
			result, err := exec.Execute(ctx, "npm link", executor.Options{Cwd: info.path})
			if err != nil {
				output.ProjectStatus(pkgName, "error", err.Error())
				continue
			}

			if result.ExitCode == 0 {
				output.ProjectStatus(pkgName, "success", "linked globally")
				linkCount++
			} else {
				output.ProjectStatus(pkgName, "error", result.Stderr)
			}
		}
	}

	output.Success(fmt.Sprintf("Created %d links", linkCount))
	return nil
}

func readPackageJSON(dir string) (*packageJSON, error) {
	pkgPath := filepath.Join(dir, "package.json")
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, err
	}

	var pkg packageJSON
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil, err
	}

	return &pkg, nil
}
