package filter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Options defines the filter criteria for project directories.
type Options struct {
	IncludeOnly    []string
	ExcludeOnly    []string
	IncludePattern *regexp.Regexp
	ExcludePattern *regexp.Regexp
}

// Apply applies filter options to a list of directories.
// Filter order: includeOnly → excludeOnly → includePattern → excludePattern.
func Apply(directories []string, opts Options) []string {
	result := make([]string, len(directories))
	copy(result, directories)

	if len(opts.IncludeOnly) > 0 {
		includeSet := toSet(opts.IncludeOnly)
		result = filterDirs(result, func(dir string) bool {
			return includeSet[dir] || includeSet[filepath.Base(dir)]
		})
	}

	if len(opts.ExcludeOnly) > 0 {
		excludeSet := toSet(opts.ExcludeOnly)
		result = filterDirs(result, func(dir string) bool {
			return !excludeSet[dir] && !excludeSet[filepath.Base(dir)]
		})
	}

	if opts.IncludePattern != nil {
		result = filterDirs(result, func(dir string) bool {
			return opts.IncludePattern.MatchString(dir)
		})
	}

	if opts.ExcludePattern != nil {
		result = filterDirs(result, func(dir string) bool {
			return !opts.ExcludePattern.MatchString(dir)
		})
	}

	return result
}

// ParseFilterList splits a comma-separated string into a trimmed slice.
// Returns nil if input is empty.
func ParseFilterList(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	var result []string
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ParseFilterPattern compiles a regex pattern string.
// Returns nil if input is empty.
func ParseFilterPattern(input string) (*regexp.Regexp, error) {
	if input == "" {
		return nil, nil
	}
	re, err := regexp.Compile(input)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %s", input)
	}
	return re, nil
}

// CreateFilterOptions creates filter options from string inputs.
func CreateFilterOptions(includeOnly, excludeOnly, includePattern, excludePattern string) (Options, error) {
	inclPattern, err := ParseFilterPattern(includePattern)
	if err != nil {
		return Options{}, err
	}
	exclPattern, err := ParseFilterPattern(excludePattern)
	if err != nil {
		return Options{}, err
	}
	return Options{
		IncludeOnly:    ParseFilterList(includeOnly),
		ExcludeOnly:    ParseFilterList(excludeOnly),
		IncludePattern: inclPattern,
		ExcludePattern: exclPattern,
	}, nil
}

// FilterFromLoopRc filters directories using a looprc ignore list.
func FilterFromLoopRc(directories, ignore []string) []string {
	if len(ignore) == 0 {
		return directories
	}
	ignoreSet := toSet(ignore)
	return filterDirs(directories, func(dir string) bool {
		return !ignoreSet[dir] && !ignoreSet[filepath.Base(dir)]
	})
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

func filterDirs(dirs []string, pred func(string) bool) []string {
	var result []string
	for _, d := range dirs {
		if pred(d) {
			result = append(result, d)
		}
	}
	return result
}
