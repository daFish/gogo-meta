package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new gogo-meta repository",
		RunE:  runInit,
	}
	cmd.Flags().BoolP("force", "f", false, "Overwrite existing config file")
	cmd.Flags().String("format", "json", "Config file format (json or yaml)")
	return cmd
}

func runInit(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr != "json" && formatStr != "yaml" {
		return fmt.Errorf("invalid format %q. Use \"json\" or \"yaml\"", formatStr)
	}
	format := config.ConfigFormat(formatStr)

	force, _ := cmd.Flags().GetBool("force")

	var existingFiles []string
	for _, candidate := range config.MetaFileCandidates {
		if _, err := os.Stat(filepath.Join(cwd, candidate)); err == nil {
			existingFiles = append(existingFiles, candidate)
		}
	}

	if len(existingFiles) > 0 {
		if !force {
			return fmt.Errorf("%s file already exists. Use --force to overwrite", existingFiles[0])
		}
		for _, candidate := range existingFiles {
			if err := os.Remove(filepath.Join(cwd, candidate)); err != nil {
				return err
			}
		}
		output.Warning("Overwriting existing config file")
	}

	cfg := config.CreateDefaultConfig()
	if err := config.WriteMetaConfig(cwd, cfg, format); err != nil {
		return err
	}

	filename := config.FilenameForFormat(format)
	output.Success(fmt.Sprintf("Created %s file in %s", filename, cwd))
	output.Info("Add projects with: gogo project import <folder> <repo-url>")
	return nil
}
