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
package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BytemanHelper provides Byteman infrastructure for Go integration tests.
// It manages Byteman rule files and configuration for injecting failures into Java processes.
type BytemanHelper struct {
	exportDir    string
	ruleFilePath string
	bytemanJar   string
	rules        []string // Raw rule text
}

// NewBytemanHelper creates a new Byteman helper for the given export directory.
// It validates that Byteman is installed by checking for BYTEMAN_JAR environment variable.
// Returns an error if Byteman is not found.
func NewBytemanHelper(exportDir string) (*BytemanHelper, error) {
	bytemanJar := os.Getenv("BYTEMAN_JAR")
	if bytemanJar == "" {
		return nil, fmt.Errorf("BYTEMAN_JAR environment variable not set. Please set it to the path of byteman.jar")
	}

	if _, err := os.Stat(bytemanJar); err != nil {
		return nil, fmt.Errorf("byteman.jar not found at %s: %w", bytemanJar, err)
	}

	return &BytemanHelper{
		exportDir:    exportDir,
		ruleFilePath: filepath.Join(exportDir, "byteman-rules.btm"),
		bytemanJar:   bytemanJar,
		rules:        []string{},
	}, nil
}

// AddRule adds a raw Byteman rule text (full RULE...ENDRULE block).
// Use this when you need full control over the rule syntax.
func (b *BytemanHelper) AddRule(ruleText string) {
	b.rules = append(b.rules, ruleText)
}

// AddRuleFromBuilder adds a rule created using the fluent RuleBuilder API.
// This is the recommended way to create rules for better readability.
func (b *BytemanHelper) AddRuleFromBuilder(builder *RuleBuilder) {
	b.AddRule(builder.Build())
}

// WriteRules writes all added rules to the Byteman rule file.
// This must be called before running tests with Byteman.
func (b *BytemanHelper) WriteRules() error {
	content := strings.Join(b.rules, "\n\n")
	return os.WriteFile(b.ruleFilePath, []byte(content), 0644)
}

// GetEnv returns environment variables needed to enable Byteman for Java processes.
// The returned map should be merged with the test command's environment.
// It configures JAVA_OPTS to load the Byteman agent with the rule file.
func (b *BytemanHelper) GetEnv() map[string]string {
	return map[string]string{
		"JAVA_OPTS": fmt.Sprintf("-javaagent:%s=script:%s,boot:%s",
			b.bytemanJar, b.ruleFilePath, b.bytemanJar),
	}
}

// VerifyInjection checks if a Byteman injection occurred by searching Debezium logs.
// It looks for the given regex pattern in the most recent Debezium log file.
// Returns true if the pattern is found, false otherwise.
func (b *BytemanHelper) VerifyInjection(pattern string) (bool, error) {
	logPattern := filepath.Join(b.exportDir, "logs", "debezium-*.log")
	matches, err := filepath.Glob(logPattern)
	if err != nil || len(matches) == 0 {
		return false, fmt.Errorf("debezium log not found in %s", logPattern)
	}

	// Use the first match (usually there's only one)
	content, err := os.ReadFile(matches[0])
	if err != nil {
		return false, fmt.Errorf("failed to read log file %s: %w", matches[0], err)
	}

	matched, err := regexp.MatchString(pattern, string(content))
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
	}

	return matched, nil
}

//
// RuleBuilder - Fluent API for building Byteman rules
//

// RuleBuilder provides a fluent API for constructing Byteman rules.
// Use NewRule() to start building a rule, then chain methods to configure it.
type RuleBuilder struct {
	name      string
	class     string
	method    string
	location  string
	condition string
	action    string
}

// NewRule starts building a new Byteman rule with the given name.
// The name should be descriptive and unique within a rule file.
func NewRule(name string) *RuleBuilder {
	return &RuleBuilder{
		name:      name,
		condition: "true", // Default condition
	}
}

// Class sets the target Java class name for the rule.
// Use fully qualified class names (e.g., "java.sql.Connection").
func (r *RuleBuilder) Class(className string) *RuleBuilder {
	r.class = className
	return r
}

// Method sets the target method name within the class.
func (r *RuleBuilder) Method(methodName string) *RuleBuilder {
	r.method = methodName
	return r
}

// AtEntry injects the rule at the entry point of the method (before execution).
func (r *RuleBuilder) AtEntry() *RuleBuilder {
	r.location = "AT ENTRY"
	return r
}

// AtExit injects the rule at the exit point of the method (after execution).
func (r *RuleBuilder) AtExit() *RuleBuilder {
	r.location = "AT EXIT"
	return r
}

// AtInvoke injects the rule before a specific method call within the target method.
// The methodCall parameter should be the method signature (e.g., "someMethod()").
func (r *RuleBuilder) AtInvoke(methodCall string) *RuleBuilder {
	r.location = fmt.Sprintf("AT INVOKE %s", methodCall)
	return r
}

// AtLine injects the rule at a specific line number in the source code.
// Note: This requires debug symbols and is fragile as line numbers can change.
func (r *RuleBuilder) AtLine(lineNum int) *RuleBuilder {
	r.location = fmt.Sprintf("AT LINE %d", lineNum)
	return r
}

// AtMarker targets a BytemanMarkers.checkpoint() call with the given name.
// This is the recommended approach for stable, self-documenting injection points.
// Example: AtMarker("before-poll") targets BytemanMarkers.checkpoint("before-poll")
func (r *RuleBuilder) AtMarker(checkpointName string) *RuleBuilder {
	r.class = "com.yugabyte.ybvoyager.BytemanMarkers"
	r.method = "checkpoint"
	r.location = "AT ENTRY"
	r.condition = fmt.Sprintf(`$1.equals("%s")`, checkpointName)
	return r
}

// AtCDCMarker targets a BytemanMarkers.cdc() call with the given event name.
// Example: AtCDCMarker("before-poll") targets BytemanMarkers.cdc("before-poll")
func (r *RuleBuilder) AtCDCMarker(eventName string) *RuleBuilder {
	r.class = "com.yugabyte.ybvoyager.BytemanMarkers"
	r.method = "cdc"
	r.location = "AT ENTRY"
	r.condition = fmt.Sprintf(`$1.equals("%s")`, eventName)
	return r
}

// AtSnapshotMarker targets a BytemanMarkers.snapshot() call with the given phase name.
// Example: AtSnapshotMarker("before-complete") targets BytemanMarkers.snapshot("before-complete")
func (r *RuleBuilder) AtSnapshotMarker(phaseName string) *RuleBuilder {
	r.class = "com.yugabyte.ybvoyager.BytemanMarkers"
	r.method = "snapshot"
	r.location = "AT ENTRY"
	r.condition = fmt.Sprintf(`$1.equals("%s")`, phaseName)
	return r
}

// AtDBMarker targets a BytemanMarkers.db() call with the given operation name.
// Example: AtDBMarker("connect") targets BytemanMarkers.db("connect")
func (r *RuleBuilder) AtDBMarker(operationName string) *RuleBuilder {
	r.class = "com.yugabyte.ybvoyager.BytemanMarkers"
	r.method = "db"
	r.location = "AT ENTRY"
	r.condition = fmt.Sprintf(`$1.equals("%s")`, operationName)
	return r
}

// If sets the condition for when the rule should trigger.
// The condition is a Byteman expression (e.g., "incrementCounter(\"count\") == 5").
// Default condition is "true" (always trigger).
func (r *RuleBuilder) If(condition string) *RuleBuilder {
	r.condition = condition
	return r
}

// Do sets the action to perform when the rule triggers.
// The action is Byteman code (e.g., "traceln(\"message\"); throw new Exception()").
func (r *RuleBuilder) Do(action string) *RuleBuilder {
	r.action = action
	return r
}

// ThrowException is a shorthand for throwing an exception with a message.
// It automatically adds a traceln for debugging.
func (r *RuleBuilder) ThrowException(exceptionClass, message string) *RuleBuilder {
	r.action = fmt.Sprintf(`traceln(">>> BYTEMAN: %s - throwing %s");
   throw new %s("%s")`, r.name, exceptionClass, exceptionClass, message)
	return r
}

// Delay is a shorthand for injecting a delay (sleep) in seconds.
// Useful for testing timeout behavior.
func (r *RuleBuilder) Delay(seconds int) *RuleBuilder {
	r.action = fmt.Sprintf(`traceln(">>> BYTEMAN: %s - delaying %ds");
   Thread.sleep(%d)`, r.name, seconds, seconds*1000)
	return r
}

// Build generates the final Byteman rule text.
// This is called automatically by AddRuleFromBuilder().
/*
Example:
RULE "fail_connection"
CLASS "java.sql.DriverManager"
METHOD "getConnection"
AT ENTRY
IF "incrementCounter(\"connections\") == 3"
DO "traceln(\">>> BYTEMAN: Connection refused\");
   throw new java.sql.SQLException(\"Connection refused\")"
ENDRULE
*/
func (r *RuleBuilder) Build() string {
	if r.class == "" || r.method == "" || r.location == "" || r.action == "" {
		panic(fmt.Sprintf("incomplete rule '%s': must set class, method, location, and action", r.name))
	}

	return fmt.Sprintf(`RULE %s
CLASS %s
METHOD %s
%s
IF %s
DO %s
ENDRULE`, r.name, r.class, r.method, r.location, r.condition, r.action)
}
