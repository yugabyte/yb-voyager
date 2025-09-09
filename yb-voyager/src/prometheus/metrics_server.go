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
package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// StartMetricsServer starts an HTTP server to expose Prometheus metrics
func StartMetricsServer(port string) error {
	// Create HTTP handler for /metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Start server in a goroutine
	go func() {
		addr := ":" + port
		log.Infof("Starting Prometheus metrics server on port %s", port)
		log.Infof("Metrics available at http://localhost%s/metrics", addr)
		log.Infof("Session ID for this run: %s", GetSessionID())

		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Errorf("Failed to start Prometheus metrics server: %v", err)
		}
	}()

	return nil
}
