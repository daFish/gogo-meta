package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate config files and check that configured projects exist in the working copy",
		RunE:  runValidate,
	}
}

type validationResult struct {
	file  string
	valid bool
	err   string
	cfg   *config.MetaConfig
}

func runValidate(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	configFiles, err := findConfigFiles(cwd)
	if err != nil {
		return err
	}

	if len(configFiles) == 0 {
		output.Warning("No config files found in current directory")
		return nil
	}

	var results []validationResult
	for _, filename := range configFiles {
		filePath := filepath.Join(cwd, filename)
		results = append(results, validateConfigFile(filePath, filename))
	}

	configHasErrors := false
	var knownConfigs []config.MetaConfig
	var reportedFiles []string
	for _, r := range results {
		if r.valid {
			output.ProjectStatus(r.file, "success", "")
			if r.cfg != nil {
				knownConfigs = append(knownConfigs, *r.cfg)
			}
		} else {
			output.ProjectStatus(r.file, "error", r.err)
			configHasErrors = true
			reportedFiles = append(reportedFiles, filepath.Join(cwd, r.file))
		}
	}

	workingCopyHasErrors := validateWorkingCopy(cwd, knownConfigs, reportedFiles)

	if configHasErrors || workingCopyHasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

func findConfigFiles(cwd string) ([]string, error) {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return nil, err
	}

	var configFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if name == ".gogo" || strings.HasPrefix(name, ".gogo.") {
			configFiles = append(configFiles, name)
		}
	}

	sort.Strings(configFiles)
	return configFiles, nil
}

const missingDirectoryHint = "directory missing — run 'gogo migrate' if it moved, or 'gogo git update' to clone"

// validateWorkingCopy checks the merged config — cross-references between
// projects, groups and commands — plus the presence of every configured
// project directory in the working copy. It prints the problems it finds and
// returns true when there was at least one. If the cwd is not inside a meta
// repo it returns false without output.
//
// knownConfigs holds every config file found next to the primary one, so a
// group may name a project that only an unloaded overlay declares.
// reportedFiles names the files the per-file loop has already printed an error
// for; a failure blamed on one of those still counts, but is not printed twice.
func validateWorkingCopy(cwd string, knownConfigs []config.MetaConfig, reportedFiles []string) bool {
	metaPath, err := config.FindMetaFileUp(cwd)
	if err != nil || metaPath == "" {
		return false
	}

	result, err := config.ReadMetaConfig(cwd, nil)
	if err != nil {
		// Only whole-file problems reach this branch: a meta file that cannot
		// be read or parsed — it may sit in a parent directory, out of reach of
		// the per-file loop — or an overlay named with -f. Cross-references are
		// checked further down, once the merge has succeeded.
		message := err.Error()
		var cfgErr *config.ConfigError
		if errors.As(err, &cfgErr) {
			if slices.Contains(reportedFiles, cfgErr.Path) {
				return true
			}
			message = cfgErr.Message
		}
		output.Error(message)
		return true
	}

	for _, w := range result.Warnings {
		output.Warning(w)
	}

	hasErrors := false

	// Group and command cross-references are only meaningful once every config
	// file has been merged, so they are checked here rather than per file. Like
	// the directory check below, every problem is listed at once.
	//
	// The primary config may live in a parent directory, whose overlays the cwd
	// scan never saw, so they are added here. Naming the same file twice makes
	// no difference — only the project paths and group names are collected.
	known := append(discoverConfigs(result.MetaDir), knownConfigs...)
	for _, problem := range config.ValidateReferences(result.Config, known...) {
		output.Error(problem.Error())
		hasErrors = true
	}

	projectPaths := make([]string, 0, len(result.Config.Projects))
	for p := range result.Config.Projects {
		projectPaths = append(projectPaths, p)
	}
	if len(projectPaths) == 0 {
		return hasErrors
	}
	sort.Strings(projectPaths)

	missing := false
	for _, projectPath := range projectPaths {
		projectDir := filepath.Join(result.MetaDir, projectPath)
		if !config.FileExists(projectDir) {
			output.ProjectStatus(projectPath, "error", missingDirectoryHint)
			missing = true
		}
	}

	if !missing {
		output.Success(fmt.Sprintf("All %d project directories present", len(projectPaths)))
	}

	return hasErrors || missing
}

// discoverConfigs parses the config files in dir and returns the ones that pass
// their structural checks, to widen the universe a reference check resolves
// against. Unreadable and invalid files are skipped: a file that fails its own
// checks does not get to vouch for a project path.
func discoverConfigs(dir string) []config.MetaConfig {
	filenames, err := findConfigFiles(dir)
	if err != nil {
		return nil
	}

	var configs []config.MetaConfig
	for _, filename := range filenames {
		if result := validateConfigFile(filepath.Join(dir, filename), filename); result.valid && result.cfg != nil {
			configs = append(configs, *result.cfg)
		}
	}
	return configs
}

func validateConfigFile(filePath, filename string) validationResult {
	format := config.DetectFormat(filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return validationResult{file: filename, valid: false, err: err.Error()}
	}

	cfg, err := config.ParseConfigContent(content, format)
	if err != nil {
		return validationResult{file: filename, valid: false, err: fmt.Sprintf("Invalid %s: %v", format, err)}
	}

	if err := config.Validate(*cfg); err != nil {
		return validationResult{file: filename, valid: false, err: fmt.Sprintf("Invalid structure: %v", err)}
	}

	return validationResult{file: filename, valid: true, cfg: cfg}
}
