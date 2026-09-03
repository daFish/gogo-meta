package ssh

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// UnverifiedSSHHosts returns the SSH hosts referenced by urls that are not yet
// present in the user's known_hosts. gogo deliberately never adds host keys
// automatically: scanning and trusting whatever key the network returns would
// defeat SSH's host-key verification. Callers should warn the user and let them
// verify and add the keys, or let ssh prompt during clone.
func UnverifiedSSHHosts(urls []string) []string {
	var unverified []string
	for _, host := range ExtractUniqueSSHHosts(urls) {
		if !IsHostKnown(host) {
			unverified = append(unverified, host)
		}
	}
	return unverified
}
