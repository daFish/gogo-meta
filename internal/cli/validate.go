package cli

import (
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
		Short: "Validate all config files in the current directory",
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
		if filename == config.LoopRcFile {
			results = append(results, validateLoopRcFile(filePath))
		} else {
			results = append(results, validateConfigFile(filePath, filename))
		}
	}

	hasErrors := false
	for _, r := range results {
		if r.valid {
			output.ProjectStatus(r.file, "success", "")
		} else {
			output.ProjectStatus(r.file, "error", r.err)
			hasErrors = true
		}
	}

	if hasErrors {
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
	hasLoopRc := false

	for _, entry := range entries {
		name := entry.Name()
		if name == ".gogo" || strings.HasPrefix(name, ".gogo.") {
			configFiles = append(configFiles, name)
		}
		if name == config.LoopRcFile {
			hasLoopRc = true
		}
	}

	if hasLoopRc {
		configFiles = append(configFiles, config.LoopRcFile)
	}

	sort.Strings(configFiles)
	return configFiles, nil
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

func validateLoopRcFile(filePath string) validationResult {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return validationResult{file: config.LoopRcFile, valid: false, err: err.Error()}
	}

	rc, err := config.ParseLoopRcContent(content)
	if err != nil {
		return validationResult{file: config.LoopRcFile, valid: false, err: fmt.Sprintf("Invalid JSON: %v", err)}
	}

	if err := config.ValidateLoopRc(*rc); err != nil {
		return validationResult{file: config.LoopRcFile, valid: false, err: fmt.Sprintf("Invalid structure: %v", err)}
	}

	return validationResult{file: config.LoopRcFile, valid: true}
}
