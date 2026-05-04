package ssh

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/daFish/gogo-meta/internal/executor"
	"github.com/daFish/gogo-meta/internal/output"
)

var (
	sshURLPattern = regexp.MustCompile(`^ssh://[^@]+@([^/:]+)`)
	gitURLPattern = regexp.MustCompile(`^[^@]+@([^:]+):`)
)

// ExtractSSHHost extracts the SSH host from a git URL.
// Returns empty string for non-SSH URLs.
func ExtractSSHHost(url string) string {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "file://") {
		return ""
	}

	if matches := sshURLPattern.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
	}

	if matches := gitURLPattern.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// ExtractUniqueSSHHosts extracts unique SSH hosts from a list of URLs.
func ExtractUniqueSSHHosts(urls []string) []string {
	seen := make(map[string]bool)
	var hosts []string

	for _, url := range urls {
		host := ExtractSSHHost(url)
		if host != "" && !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}

	return hosts
}

// IsHostKnown checks if a host is in the known_hosts file.
func IsHostKnown(host string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return false
	}

	escapedHost := regexp.QuoteMeta(host)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^` + escapedHost + `[,\s]`),
		regexp.MustCompile(`(?m)^\[` + escapedHost + `\]:\d+[,\s]`),
	}

	for _, pattern := range patterns {
		if pattern.Match(content) {
			return true
		}
	}

	return false
}

// AddHostKey adds a host's SSH key to known_hosts using ssh-keyscan.
func AddHostKey(host string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	command := `ssh-keyscan -H "` + host + `" >> "` + knownHostsPath + `" 2>/dev/null`

	result := executor.ExecuteSync(command, executor.Options{
		Cwd: ".",
	})

	return result.ExitCode == 0
}

// EnsureSSHHostsKnown ensures all SSH hosts for the given URLs are in known_hosts.
func EnsureSSHHostsKnown(urls []string) (added, failed []string) {
	hosts := ExtractUniqueSSHHosts(urls)

	for _, host := range hosts {
		if !IsHostKnown(host) {
			output.Info("Adding SSH host key for " + host + "...")
			if AddHostKey(host) {
				output.Success("Added host key for " + host)
				added = append(added, host)
			} else {
				output.Error("Failed to add host key for " + host)
				failed = append(failed, host)
			}
		}
	}

	return added, failed
}
