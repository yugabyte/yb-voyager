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
package cmd

import (
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metrics"
)

// legacyProfileDefaultMetricsPorts are the fixed per-role ports that --profile used to
// start metrics on before --metrics-port existed. Preserved so --profile alone (with no
// --metrics-port/--prometheus-metrics-port) keeps working, behind a deprecation warning.
var legacyProfileDefaultMetricsPorts = map[string]int{
	constants.TARGET_DB_IMPORTER_ROLE:         9101,
	constants.IMPORT_FILE_ROLE:                9102,
	constants.SOURCE_REPLICA_DB_IMPORTER_ROLE: 9103,
	constants.SOURCE_DB_IMPORTER_ROLE:         9104,
}

// resolveMetricsPort decides whether metrics are enabled and on which port.
// A non-zero override enables metrics on that port and always wins. Otherwise, if
// legacyProfileFallback is set (--profile with no explicit port), fall back to the
// role's legacy default port. Returns disabled if neither applies.
func resolveMetricsPort(role string, override int, legacyProfileFallback bool) (int, bool) {
	if override != 0 {
		return override, true
	}
	if legacyProfileFallback {
		if port, ok := legacyProfileDefaultMetricsPorts[role]; ok {
			return port, true
		}
	}
	return 0, false
}

// startMetricsServer installs a PrometheusRecorder and starts serving /metrics
// when --metrics-port (or the deprecated --prometheus-metrics-port, or --profile's
// legacy default port) resolves to a non-zero port. It is a no-op when metrics are
// disabled. Shared by import and export commands.
func startMetricsServer(role string, migrationUUID uuid.UUID) error {
	override := metricsPort
	if override == 0 && prometheusMetricsPort != 0 {
		log.Warn("--prometheus-metrics-port is deprecated; use --metrics-port")
		override = prometheusMetricsPort
	}
	port, enabled := resolveMetricsPort(role, override, bool(perfProfile))
	if !enabled {
		return nil
	}
	if override == 0 {
		log.Warnf("starting metrics on default port %d because --profile is set; this fallback is deprecated, use --metrics-port %d explicitly instead", port, port)
	}
	rec := metrics.NewPrometheusRecorder(migrationUUID.String(), metrics.SessionID())
	metrics.SetRecorder(rec)
	srv := metrics.NewServer(port, rec.Registry())
	return srv.Start()
}
