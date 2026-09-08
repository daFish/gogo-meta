package cli

// User-facing hints that point at the command which resolves a situation.
// They live together so the advice stays consistent: every one of them names
// gogo update, the only command that cannot bounce the user on to another.
const (
	missingDirectoryHint = "directory missing — run 'gogo update' to move or clone it"
	notFoundHint         = "%s not found in working copy — run 'gogo update' to clone it"
	cloneLaterHint       = `Run "gogo update" to clone missing projects`
)
