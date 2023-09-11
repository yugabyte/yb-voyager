/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

const (
	// This constant must be updated on every release.
	YB_VOYAGER_VERSION = "1.5.0"

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
