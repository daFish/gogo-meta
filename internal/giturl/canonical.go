package giturl

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// caseInsensitiveForges serve repository paths case-insensitively, so
// "Org/Api" and "org/api" name the same repository there.
var caseInsensitiveForges = map[string]bool{
	"github.com":    true,
	"gitlab.com":    true,
	"bitbucket.org": true,
}

// Canonical reduces a git remote URL to a form in which two spellings of the
// same repository compare equal: the transport, user and port are dropped, the
// host is lowercased, and trailing slashes and a ".git" suffix are removed.
// Paths on case-insensitive forges are lowercased as well. Local paths keep
// their case and carry a "file:" prefix, so they never equal a network remote.
//
// The result is for comparison only and must never be handed to git: the
// configured URL carries the transport the user intends.
func Canonical(rawURL string) string {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return trimRepoSuffix(raw)
		}
		if u.Scheme == "file" {
			return canonicalLocal(u.Path)
		}
		return canonicalRemote(u.Hostname(), u.Path)
	}
	if host, repoPath, ok := splitSCP(raw); ok {
		return canonicalRemote(host, repoPath)
	}
	return canonicalLocal(raw)
}

// splitSCP splits an scp-style "[user@]host:path" URL. Like git, it treats the
// input as a local path when a slash precedes the first colon, and a single
// letter before the colon as a Windows drive rather than a host.
func splitSCP(raw string) (host, repoPath string, ok bool) {
	inBrackets := false
	for i, r := range raw {
		switch r {
		case '[':
			inBrackets = true
		case ']':
			inBrackets = false
		case '/':
			return "", "", false
		case ':':
			if inBrackets {
				continue
			}
			userHost := raw[:i]
			if len(userHost) < 2 {
				return "", "", false
			}
			if at := strings.LastIndex(userHost, "@"); at >= 0 {
				userHost = userHost[at+1:]
			}
			return strings.Trim(userHost, "[]"), raw[i+1:], true
		}
	}
	return "", "", false
}

func canonicalRemote(host, repoPath string) string {
	host = strings.ToLower(host)
	repoPath = strings.Trim(trimRepoSuffix(path.Clean("/"+repoPath)), "/")
	if caseInsensitiveForges[host] {
		repoPath = strings.ToLower(repoPath)
	}
	return host + "/" + repoPath
}

func canonicalLocal(localPath string) string {
	return "file:" + trimRepoSuffix(filepath.Clean(localPath))
}

func trimRepoSuffix(s string) string {
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return strings.TrimRight(s, "/")
}
