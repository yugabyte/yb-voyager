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

package migassessment

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/ux"
)

// PrintAssessmentSummary prints the final summary block with issue count and report paths.
func PrintAssessmentSummary(issueCount int, jsonPath, htmlPath string) {
	fmt.Println()
	ux.PrintSeparator()
	fmt.Println()
	ux.CheckColor.Printf("✓ Assessment completed successfully\n")
	fmt.Println()
	ux.DimColor.Printf("  Issues detected: ")
	if issueCount > 0 {
		color.New(color.FgYellow).Printf("%d\n", issueCount)
	} else {
		ux.CheckColor.Printf("%d\n", issueCount)
	}
	ux.DimColor.Print("  JSON report: ")
	ux.PathColor.Println(jsonPath)
	ux.DimColor.Print("  HTML report: ")
	ux.PathColor.Println(htmlPath)
	fmt.Println()
}

// PrintAssessmentFailure prints a failure message at the end of the assessment.
func PrintAssessmentFailure(errMsg string) {
	fmt.Println()
	ux.PrintSeparator()
	fmt.Println()
	ux.FailColor.Printf("✗ Assessment failed: %s\n", errMsg)
	fmt.Println()
}
