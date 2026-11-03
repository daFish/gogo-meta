package giturl

import "strings"

// Redact removes the user info from a "scheme://" URL so the URL can be shown
// to the user: a token may sit in the password or in the user name. It works on
// the raw string, so a URL that net/url rejects is redacted too. scp-style
// "user@host:path" URLs carry no secret and are returned unchanged.
func Redact(rawURL string) string {
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return rawURL
	}
	authorityStart := schemeEnd + len("://")
	rest := rawURL[authorityStart:]
	authority := rest
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		authority = rest[:end]
	}
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return rawURL
	}
	return rawURL[:authorityStart] + rest[at+1:]
}
