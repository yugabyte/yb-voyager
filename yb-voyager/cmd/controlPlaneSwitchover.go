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

// POC: control-plane switchover at plan-migration time.
//
// During assessment, voyager pushes events to the LOCAL yugabyted control
// plane (default 127.0.0.1:5433). At plan-migration, once the target YB
// connection has been captured and written into config.yaml, we:
//
//  1. Derive a new yugabyted-control-plane.db-conn-string from the target
//     details (same host:port, target user/password, `yugabyte` db).
//  2. Initialise a fresh YugabyteD control-plane client against that target.
//  3. Re-derive the two assessment events from on-disk state and re-emit
//     them against the target so the target's UI shows the assessment
//     history.
//  4. Rewrite the config file's yugabyted-control-plane.db-conn-string
//     line so all subsequent voyager invocations push to (and link to)
//     the target.
//  5. Stamp MSR.ControlPlaneSwitched=true so we don't repeat the switch.
//
// Scope: POC. Hard-fails on any step. No probing of target's UI availability,
// no fallback if target doesn't run yugabyted UI, no honoring of any prior
// --control-plane override. Live-migration / fall-forward / fall-back cases
// are out of scope for now.

package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp/yugabyted"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// switchControlPlaneToTarget performs the local->target yugabyted-CP switch.
// Hard-fails (via utils.ErrExit) on any error.
func switchControlPlaneToTarget(target *parsedConnInfo, configFilePath string) {
	if target == nil {
		utils.ErrExit("control-plane switchover: target connection info is nil")
	}

	// plan-migration's PersistentPreRun doesn't initialize metaDB or
	// migrationUUID by default. Lazy-init both so we can read MSR and
	// fill BaseEvent.MigrationUUID downstream.
	if metaDB == nil && metaDBIsCreated(exportDir) {
		metaDB = initMetaDB(exportDir)
	}
	if metaDB == nil {
		utils.ErrExit("control-plane switchover: metaDB not available at %s", exportDir)
	}
	// Silence retrieveMigrationUUID's "migrationID: ..." console print.
	prevQuiet := os.Getenv("VOYAGER_QUIET_STARTUP")
	_ = os.Setenv("VOYAGER_QUIET_STARTUP", "1")
	defer os.Setenv("VOYAGER_QUIET_STARTUP", prevQuiet)
	if err := retrieveMigrationUUID(); err != nil {
		utils.ErrExit("control-plane switchover: retrieve migration UUID: %v", err)
	}

	// Idempotency: if we've already switched, do nothing. plan-migration may
	// re-run (e.g. to edit target details); the switch should fire exactly once.
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("control-plane switchover: read MSR: %v", err)
	}
	if msr != nil && msr.ControlPlaneSwitched {
		log.Info("control-plane already switched to target; skipping re-emit")
		return
	}

	// Only meaningful when the configured control plane is yugabyted.
	if getControlPlaneType() != YUGABYTED {
		log.Infof("control-plane type is %q (not yugabyted); skipping switchover",
			getControlPlaneType())
		return
	}

	targetConnStr := buildTargetControlPlaneConnString(target)

	fmt.Println()
	fmt.Println("  " + dimStyle.Render("Switching migration UI to target YugabyteDB..."))

	// Suppress noisy CP init logs (CREATE TABLE / ALTER TABLE / event-publish
	// info messages) for the duration of the switchover. Restore at the end.
	prevLogOut := log.StandardLogger().Out
	log.StandardLogger().SetOutput(io.Discard)
	defer log.StandardLogger().SetOutput(prevLogOut)

	// Stand up a fresh yugabyted CP client pointed at the target. We temporarily
	// flip YUGABYTED_DB_CONN_STRING (which yugabyted.New / Init reads) and
	// restore it afterwards — the permanent flip is done by rewriting the
	// config file at the end, so the *next* command picks it up.
	previousConnStr := os.Getenv("YUGABYTED_DB_CONN_STRING")
	if err := os.Setenv("YUGABYTED_DB_CONN_STRING", targetConnStr); err != nil {
		log.StandardLogger().SetOutput(prevLogOut)
		utils.ErrExit("control-plane switchover: set env: %v", err)
	}

	targetCP := yugabyted.New(exportDir)
	if err := targetCP.Init(); err != nil {
		_ = os.Setenv("YUGABYTED_DB_CONN_STRING", previousConnStr)
		log.StandardLogger().SetOutput(prevLogOut)
		utils.ErrExit("control-plane switchover: init target yugabyted CP at %s: %v",
			redactConnStr(targetConnStr), err)
	}

	if err := reemitAssessmentEvents(targetCP); err != nil {
		_ = os.Setenv("YUGABYTED_DB_CONN_STRING", previousConnStr)
		log.StandardLogger().SetOutput(prevLogOut)
		utils.ErrExit("control-plane switchover: re-emit assessment events: %v", err)
	}

	// Drain in-flight events on the target CP before we declare the switch done.
	targetCP.Finalize()

	// Persist the new connection string in the config file. Subsequent voyager
	// invocations will read it via the normal config path and set the env var
	// themselves.
	if err := rewriteCPConnStringInConfig(configFilePath, targetConnStr); err != nil {
		utils.ErrExit("control-plane switchover: rewrite config: %v", err)
	}

	if err := metaDB.UpdateMigrationStatusRecord(func(r *metadb.MigrationStatusRecord) {
		r.ControlPlaneSwitched = true
	}); err != nil {
		utils.ErrExit("control-plane switchover: stamp MSR: %v", err)
	}

	fmt.Println("    " + successStyle.Render("✓") + " UI now points at " + dimStyle.Render(fmt.Sprintf("http://%s:15433/migrations?migration_uuid=%s", target.Host, migrationUUID.String())))
}

// buildTargetControlPlaneConnString builds a postgres URL for the target
// yugabyted's control-plane DB. Reuses the target YSQL credentials; connects
// to the `yugabyte` system database where yugabyted-CP stores its tables.
// Password is URL-encoded to survive special characters.
func buildTargetControlPlaneConnString(t *parsedConnInfo) string {
	host := t.Host
	port := t.Port
	if port == 0 {
		port = 5433
	}
	user := t.User
	if user == "" {
		user = "yugabyte"
	}
	password := t.Password
	if password == "" {
		password = "yugabyte"
	}
	u := &url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   "/yugabyte",
	}
	return u.String()
}

// reemitAssessmentEvents loads the on-disk assessment report into the
// assessmentReport global and re-fires MigrationAssessmentStarted +
// MigrationAssessmentCompleted against the supplied (target) CP client.
// Returns an error if assessment was never run (no on-disk report) — caller
// decides whether that's fatal.
func reemitAssessmentEvents(targetCP *yugabyted.YugabyteD) error {
	reportPath := filepath.Join(exportDir, "assessment", "reports",
		fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME))

	if !utils.FileOrFolderExists(reportPath) {
		// Per Q5c: no assessment run -> nothing to re-emit. The switch itself
		// still happens; the target UI just starts empty.
		log.Infof("no on-disk assessment report at %s; skipping event re-emit", reportPath)
		return nil
	}

	report, err := ParseJSONToAssessmentReport(reportPath)
	if err != nil {
		return fmt.Errorf("parse assessment report at %s: %w", reportPath, err)
	}
	assessmentReport = *report

	// The YBD event builder calls assessmentReport.GetTotalColocatedSize(source.DBType)
	// — populate source.DBType from MSR so size-calc doesn't fail with
	// "dbType is not yet supported" log noise on stdout.
	msr, err := metaDB.GetMigrationStatusRecord()
	if err == nil && msr != nil && msr.SourceDBConf != nil {
		source.DBType = msr.SourceDBConf.DBType
		source.DBSize = msr.SourceDBConf.DBSize
	}

	// MigrationAssessmentStarted is a bare marker — no payload.
	startedEv := createMigrationAssessmentStartedEvent()
	targetCP.MigrationAssessmentStarted(startedEv)

	// MigrationAssessmentCompleted carries the YBD-flavoured payload built
	// from the (now-loaded) assessmentReport global.
	completedEv := createMigrationAssessmentCompletedEventForYugabyteD()
	targetCP.MigrationAssessmentCompleted(completedEv)

	return nil
}

// rewriteCPConnStringInConfig finds the existing yugabyted-control-plane
// db-conn-string line in config.yaml and replaces it with newConnStr. The
// config file is YAML but written by templating; we keep edits surgical (single
// line replace) to match how the rest of the file is mutated.
func rewriteCPConnStringInConfig(configFilePath, newConnStr string) error {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	content := string(data)

	// Find the existing "db-conn-string:" line inside the yugabyted-control-plane
	// section and swap its value. The line is uniquely identifiable because no
	// other section uses db-conn-string today.
	lines := strings.Split(content, "\n")
	replaced := false
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "db-conn-string:") {
			indent := ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))]
			lines[i] = fmt.Sprintf("%sdb-conn-string: %s", indent, newConnStr)
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("could not find db-conn-string: line in %s", configFilePath)
	}

	return os.WriteFile(configFilePath, []byte(strings.Join(lines, "\n")), 0644)
}

// enforceSwitchedCPEnvVar overrides YUGABYTED_DB_CONN_STRING with the
// config-file value when MSR.ControlPlaneSwitched is true. Called from
// root.go's PersistentPreRun *after* metaDB has been initialized but before
// setControlPlane runs.
//
// Why: cmd/config.go's env-loading rule is "shell env wins over config" so
// any user with `export YUGABYTED_DB_CONN_STRING=...` in their shell rcfile
// would have the post-switchover config value silently ignored. Once we've
// switched, the config file is authoritative — force the env to match.
func enforceSwitchedCPEnvVar() {
	if metaDB == nil {
		return
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil || msr == nil || !msr.ControlPlaneSwitched {
		return
	}
	if cfgFile == "" {
		return
	}
	v := viper.New()
	v.SetConfigFile(cfgFile)
	if err := v.ReadInConfig(); err != nil {
		return
	}
	configCP := v.GetString("yugabyted-control-plane.db-conn-string")
	if configCP == "" {
		return
	}
	if os.Getenv("YUGABYTED_DB_CONN_STRING") == configCP {
		return
	}
	_ = os.Setenv("YUGABYTED_DB_CONN_STRING", configCP)
	log.Infof("control-plane switched; overriding YUGABYTED_DB_CONN_STRING from config")
}

// redactConnStr replaces the password component of a postgres URL with ****
// for safe inclusion in error messages and logs.
func redactConnStr(connStr string) string {
	u, err := url.Parse(connStr)
	if err != nil {
		return connStr
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "****")
	}
	return u.String()
}

// Ensure the cp package is imported for its side-effecting type aliases used
// elsewhere in this file's exported helpers — Go would otherwise drop the
// import.
var _ = cp.MigrationAssessmentStartedEvent{}
