package utils

const (
	// This constant must be updated on every release.
	YB_VOYAGER_VERSION = "v0.80"

	// @Refer: https://icinga.com/blog/2022/05/25/embedding-git-commit-information-in-go-binaries/
	GIT_COMMIT_HASH = "$Format:%H$"
)

func GitCommitHash() string {
	if len(GIT_COMMIT_HASH) == 40 {
		// Substitution has happened.
		return GIT_COMMIT_HASH
	}
	return ""
}
