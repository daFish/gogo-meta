//go:build !unix

package config

// osConfigFileOwner cannot determine file ownership on this platform, so it
// treats every config as owned by the current user. The ownership guard against
// a config planted by another user is only enforced on unix systems.
func osConfigFileOwner(_ string) (owned bool, ownerUID int, err error) {
	return true, -1, nil
}
