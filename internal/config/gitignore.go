package config

import (
	"os"
	"path/filepath"
	"strings"
)

// AddToGitignore adds an entry to the .gitignore file in the given directory.
// Returns true if the entry was added, false if it already existed.
// Creates the .gitignore file if it doesn't exist.
func AddToGitignore(metaDir, entry string) (bool, error) {
	gitignorePath := filepath.Join(metaDir, ".gitignore")

	if FileExists(gitignorePath) {
		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			return false, err
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == entry {
				return false, nil
			}
		}

		suffix := ""
		if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
			suffix = "\n"
		}

		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return false, err
		}
		defer func() { _ = f.Close() }()

		_, err = f.WriteString(suffix + entry + "\n")
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return true, os.WriteFile(gitignorePath, []byte(entry+"\n"), 0o644)
}

// RemoveFromGitignore removes the line matching entry (after trimming) from the
// .gitignore in metaDir. Returns true if a line was removed, false if the file
// is absent or contained no matching line.
func RemoveFromGitignore(metaDir, entry string) (bool, error) {
	gitignorePath := filepath.Join(metaDir, ".gitignore")

	if !FileExists(gitignorePath) {
		return false, nil
	}

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != entry {
			filtered = append(filtered, line)
		}
	}

	if len(filtered) == len(lines) {
		return false, nil
	}

	return true, os.WriteFile(gitignorePath, []byte(strings.Join(filtered, "\n")), 0o644) // #nosec G703
}
