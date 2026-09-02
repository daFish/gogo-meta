package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	for _, r := range results {
		if r.valid {
			output.ProjectStatus(r.file, "success", "")
		} else {
			output.ProjectStatus(r.file, "error", r.err)
			configHasErrors = true
		}
	}

	workingCopyHasErrors := validateWorkingCopy(cwd)

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
func validateWorkingCopy(cwd string) bool {
	metaPath, err := config.FindMetaFileUp(cwd)
	if err != nil || metaPath == "" {
		return false
	}

	result, err := config.ReadMetaConfig(cwd, nil)
	if err != nil {
		// The per-file checks already reported syntax and structure problems;
		// what surfaces here are cross-file issues such as a group referring to
		// a project defined nowhere. The path is left out because the offending
		// entry may come from any of the merged files.
		message := err.Error()
		var cfgErr *config.ConfigError
		if errors.As(err, &cfgErr) {
			message = cfgErr.Message
		}
		output.Error(message)
		return true
	}

	hasErrors := false

	// Group and command cross-references are only meaningful once every config
	// file has been merged, so they are checked here rather than per file.
	if err := config.ValidateReferences(result.Config); err != nil {
		output.Error(err.Error())
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

	return validationResult{file: filename, valid: true}
}
