package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

type projectInfo struct {
	path    string
	pkgJSON packageJSON
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

	filterOpts, err := resolveFilterOptionsWithConfig(cmd, &configResult.Config)
	if err != nil {
		return err
	}

	projectPaths := config.GetProjectPaths(configResult.Config)
	projectPaths = filter.Apply(projectPaths, filterOpts)

	if len(projectPaths) == 0 {
		output.Warning("No projects match the specified filters")
		return nil
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

	if allFlag {
		linkCount = linkAllProjects(projectPackages)
	} else {
		exec := executor.NewShellExecutor()
		ctx := cmd.Context()
		for pkgName, info := range projectPackages {
			output.Info(fmt.Sprintf("Creating global link for %s...", pkgName))
			result, err := exec.ExecuteArgs(ctx, "npm", []string{"link"}, executor.Options{Cwd: info.path})
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

func linkAllProjects(projectPackages map[string]projectInfo) int {
	linkCount := 0
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

			nodeModulesPath, err := safeLinkTarget(consumer.path, depName)
			if err != nil {
				output.Error(fmt.Sprintf("Skipping unsafe link for %q: %v", depName, err))
				continue
			}

			parentDir := filepath.Dir(nodeModulesPath)
			if err := ensureRealDirWithin(consumer.path, parentDir); err != nil {
				output.Error(fmt.Sprintf("Skipping unsafe link for %q: %v", depName, err))
				continue
			}

			_ = os.RemoveAll(nodeModulesPath)

			if err := os.Symlink(provider.path, nodeModulesPath); err != nil {
				output.Error(fmt.Sprintf("Failed to create symlink: %s -> %s", nodeModulesPath, provider.path))
				continue
			}

			output.ProjectStatus(consumerName, "success", fmt.Sprintf("linked %s", depName))
			linkCount++
		}
	}
	return linkCount
}

// safeLinkTarget resolves depName to a path inside consumerPath/node_modules,
// rejecting names that are empty, absolute, or contain a ".." segment.
func safeLinkTarget(consumerPath, depName string) (string, error) {
	if depName == "" {
		return "", fmt.Errorf("empty dependency name")
	}
	if filepath.IsAbs(depName) {
		return "", fmt.Errorf("must be a relative name")
	}
	for _, segment := range strings.Split(filepath.ToSlash(depName), "/") {
		if segment == ".." {
			return "", fmt.Errorf("must not contain a %q path segment", "..")
		}
	}

	base := filepath.Join(consumerPath, "node_modules")
	target := filepath.Join(base, depName)

	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("resolves outside node_modules")
	}
	return target, nil
}

// ensureRealDirWithin creates dir and any missing parents beneath root as real
// directories, refusing to traverse an existing symlink so a malicious
// node_modules cannot redirect the later RemoveAll/Symlink outside the tree.
func ensureRealDirWithin(root, dir string) error {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes %q", dir, root)
	}

	current := root
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		if segment == "." || segment == "" {
			continue
		}
		current = filepath.Join(current, segment)

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(current, 0o755); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to follow symlink at %q", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("%q is not a directory", current)
		}
	}
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
