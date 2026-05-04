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

	if fileExists(gitignorePath) {
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
