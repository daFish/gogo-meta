package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	MetaFile   = ".gogo"
	LoopRcFile = ".looprc"
)

var MetaFileCandidates = []string{".gogo", ".gogo.yaml", ".gogo.yml"}

// ConfigFormat represents the format of a config file.
type ConfigFormat string

const (
	FormatJSON ConfigFormat = "json"
	FormatYAML ConfigFormat = "yaml"
)

// CommandConfig represents a command that can be either a simple string or a detailed object.
// When unmarshaled from a plain string, only Cmd is set.
type CommandConfig struct {
	Cmd            string   `json:"cmd" yaml:"cmd"`
	Description    string   `json:"description,omitempty" yaml:"description,omitempty"`
	Parallel       *bool    `json:"parallel,omitempty" yaml:"parallel,omitempty"`
	Concurrency    *int     `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	IncludeOnly    []string `json:"includeOnly,omitempty" yaml:"includeOnly,omitempty"`
	ExcludeOnly    []string `json:"excludeOnly,omitempty" yaml:"excludeOnly,omitempty"`
	IncludePattern string   `json:"includePattern,omitempty" yaml:"includePattern,omitempty"`
	ExcludePattern string   `json:"excludePattern,omitempty" yaml:"excludePattern,omitempty"`
}

func (c *CommandConfig) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Cmd = s
		return nil
	}

	// Fall back to object.
	type Alias CommandConfig
	var obj Alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*c = CommandConfig(obj)
	return nil
}

func (c CommandConfig) MarshalJSON() ([]byte, error) {
	// If only Cmd is set, marshal as a plain string.
	if c.Description == "" && c.Parallel == nil && c.Concurrency == nil &&
		c.IncludeOnly == nil && c.ExcludeOnly == nil &&
		c.IncludePattern == "" && c.ExcludePattern == "" {
		return json.Marshal(c.Cmd)
	}
	type Alias CommandConfig
	return json.Marshal(Alias(c))
}

func (c *CommandConfig) UnmarshalYAML(value *yaml.Node) error {
	// Try string first.
	if value.Kind == yaml.ScalarNode {
		c.Cmd = value.Value
		return nil
	}

	// Fall back to object.
	type Alias CommandConfig
	var obj Alias
	if err := value.Decode(&obj); err != nil {
		return err
	}
	*c = CommandConfig(obj)
	return nil
}

func (c CommandConfig) MarshalYAML() (any, error) {
	// If only Cmd is set, marshal as a plain string.
	if c.Description == "" && c.Parallel == nil && c.Concurrency == nil &&
		c.IncludeOnly == nil && c.ExcludeOnly == nil &&
		c.IncludePattern == "" && c.ExcludePattern == "" {
		return c.Cmd, nil
	}
	type Alias CommandConfig
	return Alias(c), nil
}

// MetaConfig represents the .gogo configuration file.
type MetaConfig struct {
	Projects map[string]string        `json:"projects" yaml:"projects"`
	Ignore   []string                 `json:"ignore" yaml:"ignore"`
	Commands map[string]CommandConfig `json:"commands,omitempty" yaml:"commands,omitempty"`
}

// LoopRc represents the .looprc configuration file.
type LoopRc struct {
	Ignore []string `json:"ignore" yaml:"ignore"`
}

// MetaConfigResult is the result of reading a meta config file.
type MetaConfigResult struct {
	Config  MetaConfig
	Format  ConfigFormat
	MetaDir string
}

// ConfigError represents a configuration error with an optional file path.
type ConfigError struct {
	Message string
	Path    string
}

func (e *ConfigError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// ResolvedCommand is a command after normalization.
type ResolvedCommand = CommandConfig

// DefaultIgnore is the default ignore list for new configs.
var DefaultIgnore = []string{".git", "node_modules", ".vagrant", ".vscode"}

var overlayFiles []string

func SetOverlayFiles(files []string) {
	overlayFiles = files
}

func GetOverlayFiles() []string {
	return overlayFiles
}

// DetectFormat returns the config format based on file extension.
func DetectFormat(filePath string) ConfigFormat {
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		return FormatYAML
	}
	return FormatJSON
}

// FilenameForFormat returns the filename for a given format.
func FilenameForFormat(format ConfigFormat) string {
	if format == FormatYAML {
		return ".gogo.yaml"
	}
	return ".gogo"
}

func parseContent(content []byte, format ConfigFormat) (*MetaConfig, error) {
	var config MetaConfig
	var err error
	if format == FormatYAML {
		err = yaml.Unmarshal(content, &config)
	} else {
		err = json.Unmarshal(content, &config)
	}
	if err != nil {
		return nil, err
	}
	applyDefaults(&config)
	return &config, nil
}

func serializeContent(config *MetaConfig, format ConfigFormat) ([]byte, error) {
	if format == FormatYAML {
		return yaml.Marshal(config)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func applyDefaults(config *MetaConfig) {
	if config.Projects == nil {
		config.Projects = make(map[string]string)
	}
	if config.Ignore == nil {
		config.Ignore = append([]string{}, DefaultIgnore...)
	}
}

// Validate checks that a MetaConfig is valid.
func Validate(config MetaConfig) error {
	if config.Projects == nil {
		return errors.New("projects is required")
	}
	for name, cmd := range config.Commands {
		if cmd.Cmd == "" {
			return fmt.Errorf("command %q: cmd is required", name)
		}
		if cmd.Concurrency != nil && *cmd.Concurrency <= 0 {
			return fmt.Errorf("command %q: concurrency must be a positive integer", name)
		}
	}
	return nil
}

// ValidateLoopRc checks that a LoopRc is valid.
func ValidateLoopRc(rc LoopRc) error {
	// LoopRc just has an ignore list, no special validation needed.
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FindFileUp searches for a file by walking up the directory tree.
func FindFileUp(filename, startDir string) (string, error) {
	currentDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		filePath := filepath.Join(currentDir, filename)
		if fileExists(filePath) {
			return filePath, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", nil
		}
		currentDir = parentDir
	}
}

// FindMetaFileUp searches for a .gogo config file by walking up the directory tree.
func FindMetaFileUp(startDir string) (string, error) {
	currentDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		for _, candidate := range MetaFileCandidates {
			filePath := filepath.Join(currentDir, candidate)
			if fileExists(filePath) {
				return filePath, nil
			}
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", nil
		}
		currentDir = parentDir
	}
}

// GetMetaDir returns the directory containing the .gogo config file.
func GetMetaDir(cwd string) (string, error) {
	path, err := FindMetaFileUp(cwd)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	return filepath.Dir(path), nil
}

// ReadOverlayConfig reads and parses an overlay config file.
func ReadOverlayConfig(filePath string) (*MetaConfig, error) {
	if !fileExists(filePath) {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Overlay config file not found: %s", filePath),
			Path:    filePath,
		}
	}

	format := DetectFormat(filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Failed to read overlay config file: %v", err),
			Path:    filePath,
		}
	}

	config, err := parseContent(content, format)
	if err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Invalid overlay config file: %v", err),
			Path:    filePath,
		}
	}

	if err := Validate(*config); err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Invalid overlay config file structure: %v", err),
			Path:    filePath,
		}
	}

	return config, nil
}

// ReadMetaConfig reads the .gogo config file and applies overlay files.
func ReadMetaConfig(cwd string, extraOverlayFiles []string) (*MetaConfigResult, error) {
	metaPath, err := FindMetaFileUp(cwd)
	if err != nil {
		return nil, err
	}
	if metaPath == "" {
		return nil, &ConfigError{
			Message: fmt.Sprintf("No %s file found. Run 'gogo init' to create one, or navigate to a directory with a %s file.", MetaFile, MetaFile),
		}
	}

	format := DetectFormat(metaPath)
	metaDir := filepath.Dir(metaPath)

	content, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Failed to read config file: %v", err),
			Path:    metaPath,
		}
	}

	config, err := parseContent(content, format)
	if err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Invalid config file: %v", err),
			Path:    metaPath,
		}
	}

	if err := Validate(*config); err != nil {
		return nil, &ConfigError{
			Message: fmt.Sprintf("Invalid config file structure: %v", err),
			Path:    metaPath,
		}
	}

	// Determine overlay files to merge.
	filesToMerge := overlayFiles
	if extraOverlayFiles != nil {
		filesToMerge = extraOverlayFiles
	}

	for _, overlayRelPath := range filesToMerge {
		var overlayPath string
		if filepath.IsAbs(overlayRelPath) {
			overlayPath = overlayRelPath
		} else {
			overlayPath = filepath.Join(metaDir, overlayRelPath)
		}

		overlayConfig, err := ReadOverlayConfig(overlayPath)
		if err != nil {
			return nil, err
		}
		*config = MergeConfigs(*config, *overlayConfig)
	}

	return &MetaConfigResult{
		Config:  *config,
		Format:  format,
		MetaDir: metaDir,
	}, nil
}

// WriteMetaConfig writes a config file in the specified format.
func WriteMetaConfig(dir string, config MetaConfig, format ConfigFormat) error {
	if err := Validate(config); err != nil {
		return err
	}

	filename := FilenameForFormat(format)
	metaPath := filepath.Join(dir, filename)

	content, err := serializeContent(&config, format)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	return os.WriteFile(metaPath, content, 0o644)
}

// ReadLoopRc reads and parses the .looprc file.
func ReadLoopRc(cwd string) (*LoopRc, error) {
	looprcPath, err := FindFileUp(LoopRcFile, cwd)
	if err != nil {
		return nil, err
	}
	if looprcPath == "" {
		return nil, nil
	}

	content, err := os.ReadFile(looprcPath)
	if err != nil {
		return nil, nil
	}

	var rc LoopRc
	if err := json.Unmarshal(content, &rc); err != nil {
		return nil, nil
	}
	if rc.Ignore == nil {
		rc.Ignore = []string{}
	}
	return &rc, nil
}

// MergeConfigs merges an overlay config into a base config.
func MergeConfigs(base, overlay MetaConfig) MetaConfig {
	// Merge projects (overlay wins).
	projects := make(map[string]string, len(base.Projects)+len(overlay.Projects))
	for k, v := range base.Projects {
		projects[k] = v
	}
	for k, v := range overlay.Projects {
		projects[k] = v
	}

	// Merge ignore (union, deduplicated).
	seen := make(map[string]bool)
	var ignore []string
	for _, item := range base.Ignore {
		if !seen[item] {
			seen[item] = true
			ignore = append(ignore, item)
		}
	}
	for _, item := range overlay.Ignore {
		if !seen[item] {
			seen[item] = true
			ignore = append(ignore, item)
		}
	}

	// Merge commands (overlay wins).
	var commands map[string]CommandConfig
	if base.Commands != nil || overlay.Commands != nil {
		commands = make(map[string]CommandConfig)
		for k, v := range base.Commands {
			commands[k] = v
		}
		for k, v := range overlay.Commands {
			commands[k] = v
		}
	}

	return MetaConfig{
		Projects: projects,
		Ignore:   ignore,
		Commands: commands,
	}
}

// CreateDefaultConfig creates a new default config.
func CreateDefaultConfig() MetaConfig {
	return MetaConfig{
		Projects: make(map[string]string),
		Ignore:   append([]string{}, DefaultIgnore...),
	}
}

// AddProject returns a new config with a project added.
func AddProject(config MetaConfig, path, url string) MetaConfig {
	projects := make(map[string]string, len(config.Projects)+1)
	for k, v := range config.Projects {
		projects[k] = v
	}
	projects[path] = url
	return MetaConfig{
		Projects: projects,
		Ignore:   config.Ignore,
		Commands: config.Commands,
	}
}

// RemoveProject returns a new config with a project removed.
func RemoveProject(config MetaConfig, path string) MetaConfig {
	projects := make(map[string]string, len(config.Projects))
	for k, v := range config.Projects {
		if k != path {
			projects[k] = v
		}
	}
	return MetaConfig{
		Projects: projects,
		Ignore:   config.Ignore,
		Commands: config.Commands,
	}
}

// GetProjectPaths returns sorted project paths from the config.
func GetProjectPaths(config MetaConfig) []string {
	paths := make([]string, 0, len(config.Projects))
	for k := range config.Projects {
		paths = append(paths, k)
	}
	sort.Strings(paths)
	return paths
}

// GetProjectURL returns the URL for a given project path.
func GetProjectURL(config MetaConfig, path string) (string, bool) {
	url, ok := config.Projects[path]
	return url, ok
}

// NormalizeCommand normalizes a CommandConfig (identity since Go handles this in unmarshal).
func NormalizeCommand(config CommandConfig) ResolvedCommand {
	return config
}

// GetCommand returns the resolved command for a given name.
func GetCommand(config MetaConfig, name string) (ResolvedCommand, bool) {
	if config.Commands == nil {
		return CommandConfig{}, false
	}
	cmd, ok := config.Commands[name]
	if !ok {
		return CommandConfig{}, false
	}
	return NormalizeCommand(cmd), true
}

// CommandEntry is a named command for listing.
type CommandEntry struct {
	Name    string
	Command ResolvedCommand
}

// ListCommands returns all commands from the config.
func ListCommands(config MetaConfig) []CommandEntry {
	if config.Commands == nil {
		return nil
	}
	entries := make([]CommandEntry, 0, len(config.Commands))
	for name, cmd := range config.Commands {
		entries = append(entries, CommandEntry{
			Name:    name,
			Command: NormalizeCommand(cmd),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// ParseConfigContent parses raw content (JSON or YAML) into a MetaConfig.
// Exported for use by the validate command.
func ParseConfigContent(content []byte, format ConfigFormat) (*MetaConfig, error) {
	return parseContent(content, format)
}

// ParseLoopRcContent parses raw JSON content into a LoopRc.
// Exported for use by the validate command.
func ParseLoopRcContent(content []byte) (*LoopRc, error) {
	var rc LoopRc
	if err := json.Unmarshal(content, &rc); err != nil {
		return nil, err
	}
	if rc.Ignore == nil {
		rc.Ignore = []string{}
	}
	return &rc, nil
}
