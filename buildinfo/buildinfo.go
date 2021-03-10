package buildinfo

import "fmt"

var (
	// GitCommit is set by govvv at build time.
	GitCommit = ""
	// GitBranch  is set by govvv at build time.
	GitBranch = ""
	// GitState  is set by govvv at build time.
	GitState = ""
	// GitSummary is set by govvv at build time.
	GitSummary = ""
	// BuildDate  is set by govvv at build time.
	BuildDate = ""
	// Version  is set by govvv at build time.
	Version = "git"
)

// Summary prints a summary of all build info.
func Summary() string {
	return fmt.Sprintf(
		"\tversion:\t%s\n\tbuild date:\t%s\n\tgit summary:\t%s\n\tgit branch:\t%s\n\tgit commit:\t%s\n\tgit state:\t%s",
		Version,
		BuildDate,
		GitSummary,
		GitBranch,
		GitCommit,
		GitState,
	)
}
