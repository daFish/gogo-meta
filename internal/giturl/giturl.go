// Package giturl validates git remote URLs before they are handed to git,
// blocking argument injection and dangerous remote-helper transports.
package giturl

import (
	"fmt"
	"regexp"
	"strings"
)

// transportHelper matches git's "<transport>::<address>" remote-helper syntax
// (e.g. "ext::sh -c ...", "fd::..."), which can execute arbitrary commands.
var transportHelper = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*::`)

// Validate rejects git URLs that could be misused: an empty URL, one that begins
// with "-" (argument injection into git), or a remote-helper transport such as
// "ext::". Normal schemes (https, http, ssh, git, file, ...) and scp-style
// "user@host:path" URLs are allowed.
func Validate(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("empty git URL")
	}
	if strings.HasPrefix(rawURL, "-") {
		return fmt.Errorf("git URL %q must not begin with '-'", rawURL)
	}
	if transportHelper.MatchString(rawURL) {
		return fmt.Errorf("git URL %q uses a disallowed transport helper", rawURL)
	}
	return nil
}
