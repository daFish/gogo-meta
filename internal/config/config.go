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
	MetaFile = ".gogo"
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
	Groups         []string `json:"groups,omitempty" yaml:"groups,omitempty"`
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
		c.Groups == nil && c.IncludeOnly == nil && c.ExcludeOnly == nil &&
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
		c.Groups == nil && c.IncludeOnly == nil && c.ExcludeOnly == nil &&
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
	Groups   map[string][]string      `json:"groups,omitempty" yaml:"groups,omitempty"`
	Commands map[string]CommandConfig `json:"commands,omitempty" yaml:"commands,omitempty"`
}

// MetaConfigResult is the result of reading a meta config file.
type MetaConfigResult struct {
	Config          MetaConfig
	Format          ConfigFormat
	MetaDir         string
	AppliedOverlays []AppliedOverlay
	LocalProjects   map[string]string
	Warnings        []string
}

// AppliedOverlay is an overlay config that was merged into the primary config.
type AppliedOverlay struct {
	Name  string
	Local bool
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

// LocalFilenameForPrimary returns the .gogo.local sibling filename for a primary config filename.
func LocalFilenameForPrimary(primaryFilename string) string {
	suffix := strings.TrimPrefix(primaryFilename, MetaFile)
	return MetaFile + ".local" + suffix
}

var LocalOverlayNames = []string{".gogo.local", ".gogo.local.yaml", ".gogo.local.yml"}

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

// IsSafeProjectPath reports whether p is a safe, repository-relative project
// path: non-empty, not "." or "..", not absolute, and not escaping the
// repository via a leading "..".
func IsSafeProjectPath(p string) bool {
	c := filepath.Clean(p)
	if c == "" || c == "." || c == ".." || filepath.IsAbs(c) {
		return false
	}
	return !strings.HasPrefix(c, ".."+string(filepath.Separator))
}

// Validate checks that a MetaConfig is valid.
func Validate(config MetaConfig) error {
	if config.Projects == nil {
		return errors.New("projects is required")
	}
	for path := range config.Projects {
		if !IsSafeProjectPath(path) {
			return fmt.Errorf("invalid project path %q: must be relative and stay within the repository", path)
		}
	}
	for name, paths := range config.Groups {
		if strings.TrimSpace(name) == "" {
			return errors.New("group names must not be empty")
		}
		if len(paths) == 0 {
			return fmt.Errorf("group %q: must contain at least one project", name)
		}
		for _, path := range paths {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("group %q: project paths must not be empty", name)
			}
		}
	}
	for name, cmd := range config.Commands {
		if cmd.Cmd == "" {
			return fmt.Errorf("command %q: cmd is required", name)
		}
		if cmd.Concurrency != nil && *cmd.Concurrency <= 0 {
			return fmt.Errorf("command %q: concurrency must be a positive integer", name)
		}
		for _, group := range cmd.Groups {
			if strings.TrimSpace(group) == "" {
				return fmt.Errorf("command %q: group names must not be empty", name)
			}
		}
	}
	return nil
}

// ValidateReferences checks cross-references within a config: group members
// must be known projects and commands must reference known groups. Validate,
// which runs per file, does not cover this — a single overlay file may well
// group projects that are defined in the base config.
//
// Reading a config does not apply the check either: a group that references a
// project living in an overlay that is not loaded should not break unrelated
// commands. Group references are therefore resolved when they are used (see
// ResolveGroups, which reports the same problems), and `gogo validate` runs
// this check up front.
//
// References resolve against config plus alsoKnown. `gogo validate` fills
// alsoKnown with the config files sitting next to the primary one whether or
// not they were loaded, so a group naming a project that only an unloaded
// overlay declares is not reported as broken. Only the project paths and group
// names of alsoKnown are consulted — their group members and commands take no
// part in the check.
//
// Every problem is reported, not just the first, so a config with several
// stale references does not have to be fixed one round at a time. The result
// is empty when the config is consistent, and ordered by group and command
// name.
func ValidateReferences(config MetaConfig, alsoKnown ...MetaConfig) []error {
	var problems []error

	known := config
	if len(alsoKnown) > 0 {
		known = knownRefs(append([]MetaConfig{config}, alsoKnown...))
	}

	for _, name := range GetGroupNames(config) {
		for _, path := range config.Groups[name] {
			if _, ok := lookupProject(known, path); !ok {
				problems = append(problems, fmt.Errorf("group %q: unknown project %q", name, path))
			}
		}
	}

	for _, entry := range ListCommands(config) {
		for _, group := range entry.Command.Groups {
			if _, ok := known.Groups[group]; !ok {
				problems = append(problems, fmt.Errorf("command %q: unknown group %q", entry.Name, group))
			}
		}
	}

	return problems
}

// knownRefs collects what references may resolve to: every project path and
// every group name of the given configs. Group members are deliberately not
// merged — which projects a group contains is read from the config under
// check, never from the extra ones, so an overlay redefining a group cannot
// change what that group is validated against.
func knownRefs(configs []MetaConfig) MetaConfig {
	known := MetaConfig{
		Projects: make(map[string]string),
		Groups:   make(map[string][]string),
	}
	for _, cfg := range configs {
		for path, url := range cfg.Projects {
			known.Projects[path] = url
		}
		for name := range cfg.Groups {
			known.Groups[name] = nil
		}
	}
	return known
}

func FileExists(path string) bool {
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
		if FileExists(filePath) {
			return filePath, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", nil
		}
		currentDir = parentDir
	}
}

// configFileOwner reports whether a discovered config is owned by the current
// user, plus the owner uid for diagnostics. It is a package variable so tests
// can simulate a config planted by another user.
var configFileOwner = osConfigFileOwner

// requireOwnedConfig refuses a config file gogo discovered on its own when it is
// not owned by the current user. It guards every auto-loaded config — the primary
// one and the .gogo.local overlay beside it — since both may define commands that
// gogo runs. Overlays named with -f are exempt: the user chose those files.
func requireOwnedConfig(filePath string) error {
	owned, ownerUID, err := configFileOwner(filePath)
	if err != nil {
		return err
	}
	if !owned {
		return &ConfigError{
			Message: fmt.Sprintf("refusing to use a .gogo config owned by another user (uid %d); run gogo from a directory you own, or remove this file", ownerUID),
			Path:    filePath,
		}
	}
	return nil
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
			if FileExists(filePath) {
				if err := requireOwnedConfig(filePath); err != nil {
					return "", err
				}
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
	if !FileExists(filePath) {
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

	var appliedOverlays []AppliedOverlay
	var localProjects map[string]string
	var warnings []string

	if extraOverlayFiles == nil {
		localName := LocalFilenameForPrimary(filepath.Base(metaPath))
		for _, name := range LocalOverlayNames {
			if name != localName && FileExists(filepath.Join(metaDir, name)) {
				warnings = append(warnings, fmt.Sprintf(
					"Local overlay %s exists but will not be merged (format differs from the primary config)", name))
			}
		}
		localPath := filepath.Join(metaDir, localName)
		if FileExists(localPath) {
			if err := requireOwnedConfig(localPath); err != nil {
				return nil, err
			}
			localConfig, err := ReadOverlayConfig(localPath)
			if err != nil {
				return nil, err
			}
			localProjects = localConfig.Projects
			*config = MergeConfigs(*config, *localConfig)
			appliedOverlays = append(appliedOverlays, AppliedOverlay{Name: localName, Local: true})
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
		appliedOverlays = append(appliedOverlays, AppliedOverlay{Name: overlayRelPath, Local: false})
	}

	return &MetaConfigResult{
		Config:          *config,
		Format:          format,
		MetaDir:         metaDir,
		AppliedOverlays: appliedOverlays,
		LocalProjects:   localProjects,
		Warnings:        warnings,
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

	// Merge groups (overlay wins per group name).
	var groups map[string][]string
	if base.Groups != nil || overlay.Groups != nil {
		groups = make(map[string][]string, len(base.Groups)+len(overlay.Groups))
		for k, v := range base.Groups {
			groups[k] = append([]string{}, v...)
		}
		for k, v := range overlay.Groups {
			groups[k] = append([]string{}, v...)
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
		Groups:   groups,
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
		Groups:   config.Groups,
		Commands: config.Commands,
	}
}

// RemoveProject returns a new config with a project removed. The project is
// also dropped from every group that referenced it; a group left empty is
// removed (an empty group is not a valid config), and commands referring to
// such a group lose that reference, so the result never dangles.
func RemoveProject(config MetaConfig, path string) MetaConfig {
	projects := make(map[string]string, len(config.Projects))
	for k, v := range config.Projects {
		if k != path {
			projects[k] = v
		}
	}

	var groups map[string][]string
	dropped := make(map[string]bool)
	if config.Groups != nil {
		groups = make(map[string][]string, len(config.Groups))
		for name, members := range config.Groups {
			var kept []string
			for _, member := range members {
				if member != path {
					kept = append(kept, member)
				}
			}
			if len(kept) > 0 {
				groups[name] = kept
			} else {
				dropped[name] = true
			}
		}
	}

	commands := config.Commands
	if len(dropped) > 0 && commands != nil {
		commands = make(map[string]CommandConfig, len(config.Commands))
		for name, cmd := range config.Commands {
			var keptGroups []string
			for _, group := range cmd.Groups {
				if !dropped[group] {
					keptGroups = append(keptGroups, group)
				}
			}
			cmd.Groups = keptGroups
			commands[name] = cmd
		}
	}

	return MetaConfig{
		Projects: projects,
		Ignore:   config.Ignore,
		Groups:   groups,
		Commands: commands,
	}
}

// lookupProject resolves a project reference to its canonical key in
// config.Projects. It matches the key verbatim first and falls back to a
// path-cleaned comparison, so "./api" also finds the project "api".
func lookupProject(config MetaConfig, path string) (string, bool) {
	if _, ok := config.Projects[path]; ok {
		return path, true
	}
	cleaned := filepath.Clean(path)
	for key := range config.Projects {
		if filepath.Clean(key) == cleaned {
			return key, true
		}
	}
	return "", false
}

// GetGroupNames returns the sorted group names defined in the config.
func GetGroupNames(config MetaConfig) []string {
	names := make([]string, 0, len(config.Groups))
	for name := range config.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveGroups resolves group names to the union of their project paths,
// sorted and deduplicated. An unknown group name is an error listing the
// groups that are defined.
func ResolveGroups(config MetaConfig, names []string) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string

	for _, name := range names {
		members, ok := config.Groups[name]
		if !ok {
			defined := GetGroupNames(config)
			if len(defined) == 0 {
				return nil, fmt.Errorf("unknown group %q. No groups are defined in %s file", name, MetaFile)
			}
			return nil, fmt.Errorf("unknown group %q. Available groups: %s", name, strings.Join(defined, ", "))
		}
		for _, member := range members {
			key, ok := lookupProject(config, member)
			if !ok {
				return nil, fmt.Errorf("group %q: unknown project %q", name, member)
			}
			if !seen[key] {
				seen[key] = true
				paths = append(paths, key)
			}
		}
	}

	sort.Strings(paths)
	return paths, nil
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
