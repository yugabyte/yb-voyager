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
package version

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

var LatestStable *YBVersion

var V2024_1 *YBVersion

func init() {
	var err error
	V2024_1, err = NewYBVersion("2024.1")
	if err != nil {
		utils.ErrExit("could not create version 2024.1")
	}
	LatestStable = V2024_1
}
