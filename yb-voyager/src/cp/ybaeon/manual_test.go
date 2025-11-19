//go:build manual

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
package ybaeon

//  This is currently in the codebase for testing YB-Aeon API easily.
//  This is a manual test file to verify YB-Aeon API integration.
// To run these tests:
// 1. Update the configuration variables below with your YB-Aeon credentials
// 2. Update the payload variables as needed
// 3. Run specific tests:
//    - Full integration:    go test -v -run TestYBAeonAPIIntegration ./src/cp/ybaeon/
//    - Metadata only:       go test -v -run TestYBAeonMetadataOnly ./src/cp/ybaeon/
//    - Table metrics only:  go test -v -run TestYBAeonTableMetricsOnly ./src/cp/ybaeon/
//    - Get max sequence:    go test -v -run TestYBAeonGetMaxSequence ./src/cp/ybaeon/
//    - Multiple phases:     go test -v -run TestYBAeonGetMaxSequenceMultiplePhases ./src/cp/ybaeon/
//    - Response details:    go test -v -run TestYBAeonResponseDetails ./src/cp/ybaeon/

import (
	"testing"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	cp "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
)

// ==================== CONFIGURATION VARIABLES (MODIFY THESE) ====================

var (
	// YBM Configuration - Update these with your actual values
	testDomain    = "http://10.5.64.37:9000"                                                                                                                                                                                                                                                                                                                                                            // e.g., "https://cloud.yugabyte.com" or dev/portal URLs
	testAccountID = "0e5dc8a7-7904-4b0e-a635-0951701bdb2a"                                                                                                                                                                                                                                                                                                                                              // From YBM Profile menu
	testProjectID = "755a35fb-5eac-4951-acbd-819eefaa2827"                                                                                                                                                                                                                                                                                                                                              // From YBM Profile menu
	testClusterID = "7be98ec2-d68d-4efc-bbc1-b486057fd5cb"                                                                                                                                                                                                                                                                                                                                              // From Cluster list or settings
	testAPIKey    = "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiJBcGlKd3QiLCJzdWIiOiJjOTVkNWIxMy0zN2IyLTQ4MjQtYWU3Yi04MTA1YjM5YmRmNGYiLCJhY2NvdW50SWQiOiIwZTVkYzhhNy03OTA0LTRiMGUtYTYzNS0wOTUxNzAxYmRiMmEiLCJpc3MiOiJzZ2FobG90K3ZveWFnZXJAeXVnYWJ5dGUuY29tIiwiZXhwIjo0MTAyMzU4NDAwLCJpYXQiOjE3NjE3MzUyMzUsImp0aSI6IjRkMzE5NGU1LTIzNDUtNGM3OS04M2Q0LTFhM2NkMDEyYTYwMCJ9.iPHRZynWWaRfuotrwqrZv0RtlLRawEc7fGO5fbhgQ1I" // From Security -> API Keys

	// Migration Configuration
	testMigrationUUID = uuid.MustParse("055d4355-fca1-4a6e-aa1a-25ed452361f1") // Generate a new UUID or use existing
	testExportDir     = "/home/sunil/voyager_export"
)

// Metadata Event Payload - Modify as needed
var testMetadataPayload = MigrationEventPayload{
	MigrationUUID:      testMigrationUUID,
	MigrationPhase:     4, // 1=ASSESS, 2=EXPORT_SCHEMA, 3=ANALYZE_SCHEMA, 4=EXPORT_DATA, etc.
	InvocationSequence: 5, // Will be auto-managed by the code, but you can override
	MigrationDirectory: testExportDir,
	DatabaseName:       "alter_ddls",
	SchemaName:         "public",
	HostIP:             "{\"SourceDBIP\":\"10.9.72.172\"}", // JSON string format (same as yugabyted)
	Port:               5432,
	DBVersion:          "12.19",
	Payload: map[string]interface{}{
		"VoyagerVersion":      "main",
		"TargetDBVersion":     "2024.2.1.0",
		"MigrationComplexity": "high",
	},
	PayloadVersion: PAYLOAD_VERSION,
	VoyagerClientInfo: cp.VoyagerInstance{
		IP:                 "10.150.1.75",
		OperatingSystem:    "linux",
		DiskSpaceAvailable: 37852618752,
		ExportDirectory:    testExportDir,
	},
	DBType:              "postgresql",
	Status:              "COMPLETED",
	InvocationTimestamp: time.Now().Format(time.RFC3339),
}

// Table Metrics Payload - Modify as needed
var testTableMetrics = cp.TableMetrics{
	MigrationUUID:       testMigrationUUID,
	TableName:           "users_test_table",
	SchemaName:          "public",
	MigrationPhase:      4, // Must match metadata phase
	Status:              1, // 0=NOT-STARTED, 1=IN-PROGRESS, 2=DONE, 3=COMPLETED
	CountLiveRows:       1050,
	CountTotalRows:      1500,
	InvocationTimestamp: time.Now().Format(time.RFC3339),
}

// ==================== TEST FUNCTIONS ====================

// TestYBMMetadataOnly tests only the metadata endpoint
func TestYBAeonMetadataOnly(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	config := &YBAeonConfig{
		Domain:    testDomain,
		AccountID: testAccountID,
		ProjectID: testProjectID,
		ClusterID: testClusterID,
		APIKey:    testAPIKey,
	}

	ybaeon := New(testExportDir, config)
	if err := ybaeon.Init(); err != nil {
		t.Fatalf("Failed to initialize YBM: %v", err)
	}

	t.Logf("Sending metadata event: phase=%d, sequence=%d, status=%s",
		testMetadataPayload.MigrationPhase,
		testMetadataPayload.InvocationSequence,
		testMetadataPayload.Status)

	// Send only metadata
	err := ybaeon.sendMigrationEvent(testMetadataPayload)
	if err != nil {
		t.Logf("❌ Failed to send migration metadata")
		t.Logf("Error: %v", err)

		// Parse error to show response code
		errStr := err.Error()
		if contains(errStr, "400") {
			t.Log("Response Code: 400 (Bad Request - possibly duplicate entry)")
		} else if contains(errStr, "401") {
			t.Log("Response Code: 401 (Unauthorized - check API key)")
		} else if contains(errStr, "404") {
			t.Log("Response Code: 404 (Not Found - check endpoint URL)")
		} else {
			t.Logf("Did not find response code Error: %v", err)
		}
		t.Fatalf("Test failed due to API error")
	}

	t.Log("✅ Successfully sent migration metadata event")
	t.Log("Response Code: 200/201 (Success)")
	t.Log("Check debug logs above for full response body")
}

// TestYBMAPIIntegration tests the actual YBM API calls
func TestYBAeonAPIIntegration(t *testing.T) {
	log.SetLevel(log.DebugLevel) // Enable debug logging to see API calls

	// Create YBM configuration
	config := &YBAeonConfig{
		Domain:    testDomain,
		AccountID: testAccountID,
		ProjectID: testProjectID,
		ClusterID: testClusterID,
		APIKey:    testAPIKey,
	}

	// Create YBM instance
	ybaeon := New(testExportDir, config)

	// Initialize YBM (validates config, sets up HTTP client)
	err := ybaeon.Init()
	if err != nil {
		t.Fatalf("Failed to initialize YBM: %v", err)
	}

	t.Log("✅ YBM initialized successfully")

	// Test 1: Send Migration Metadata Event
	t.Run("SendMigrationMetadata", func(t *testing.T) {
		t.Logf("Sending metadata event for phase=%d, sequence=%d, status=%s",
			testMetadataPayload.MigrationPhase,
			testMetadataPayload.InvocationSequence,
			testMetadataPayload.Status)

		err := ybaeon.sendMigrationEvent(testMetadataPayload)
		if err != nil {
			t.Errorf("❌ Failed to send migration metadata: %v", err)
			t.Logf("Error details: %v", err)
		} else {
			t.Log("✅ Successfully sent migration metadata event (Status: 200/201)")
		}
	})

	// Test 2: Send Table Metrics (PUT)
	t.Run("SendTableMetrics", func(t *testing.T) {
		t.Logf("Sending table metrics for table=%s, phase=%d, status=%d, rows=%d/%d",
			testTableMetrics.TableName,
			testTableMetrics.MigrationPhase,
			testTableMetrics.Status,
			testTableMetrics.CountLiveRows,
			testTableMetrics.CountTotalRows)

		metricsList := []cp.TableMetrics{testTableMetrics}
		err := ybaeon.sendTableMetrics(metricsList)
		if err != nil {
			t.Errorf("❌ Failed to send table metrics: %v", err)
			t.Logf("Error details: %v", err)
		} else {
			t.Log("✅ Successfully sent table metrics (Status: 200/201)")
		}
	})

	// Test 3: Update metadata with COMPLETED status
	t.Run("SendCompletedMetadata", func(t *testing.T) {
		completedPayload := testMetadataPayload
		completedPayload.Status = "COMPLETED"
		completedPayload.InvocationSequence = 2
		completedPayload.InvocationTimestamp = time.Now().Format(time.RFC3339)

		t.Logf("Sending COMPLETED metadata for phase=%d, sequence=%d",
			completedPayload.MigrationPhase,
			completedPayload.InvocationSequence)

		err := ybaeon.sendMigrationEvent(completedPayload)
		if err != nil {
			t.Errorf("❌ Failed to send completed metadata: %v", err)
			t.Logf("Error details: %v", err)
		} else {
			t.Log("✅ Successfully sent completed metadata event (Status: 200/201)")
		}
	})

	// Test 4: Update table metrics to COMPLETED
	t.Run("SendCompletedTableMetrics", func(t *testing.T) {
		completedMetrics := testTableMetrics
		completedMetrics.Status = 3 // COMPLETED
		completedMetrics.CountLiveRows = 1500
		completedMetrics.InvocationTimestamp = time.Now().Format(time.RFC3339)

		t.Logf("Sending COMPLETED table metrics for table=%s, rows=%d/%d",
			completedMetrics.TableName,
			completedMetrics.CountLiveRows,
			completedMetrics.CountTotalRows)

		err := ybaeon.sendTableMetrics([]cp.TableMetrics{completedMetrics})
		if err != nil {
			t.Errorf("❌ Failed to send completed table metrics: %v", err)
			t.Logf("Error details: %v", err)
		} else {
			t.Log("✅ Successfully sent completed table metrics (Status: 200/201)")
		}
	})

	t.Log("\n🎉 All tests completed!")
}

// TestYBMTableMetricsOnly tests only the table metrics endpoint
func TestYBAeonTableMetricsOnly(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	config := &YBAeonConfig{
		Domain:    testDomain,
		AccountID: testAccountID,
		ProjectID: testProjectID,
		ClusterID: testClusterID,
		APIKey:    testAPIKey,
	}

	ybaeon := New(testExportDir, config)
	if err := ybaeon.Init(); err != nil {
		t.Fatalf("Failed to initialize YBM: %v", err)
	}

	t.Logf("Sending table metrics: table=%s, phase=%d, status=%d, rows=%d/%d",
		testTableMetrics.TableName,
		testTableMetrics.MigrationPhase,
		testTableMetrics.Status,
		testTableMetrics.CountLiveRows,
		testTableMetrics.CountTotalRows)

	// Send only table metrics
	err := ybaeon.sendTableMetrics([]cp.TableMetrics{testTableMetrics})
	if err != nil {
		t.Logf("❌ Failed to send table metrics")
		t.Logf("Error: %v", err)

		// Parse error to show response code
		errStr := err.Error()
		if contains(errStr, "400") {
			t.Log("Response Code: 400 (Bad Request - validation error)")
		} else if contains(errStr, "401") {
			t.Log("Response Code: 401 (Unauthorized - check API key)")
		} else if contains(errStr, "404") {
			t.Log("Response Code: 404 (Not Found - check endpoint URL)")
		}
		t.Fatalf("Test failed due to API error")
	}

	t.Log("✅ Successfully sent table metrics")
	t.Log("Response Code: 200/201 (Success)")
	t.Log("Check debug logs above for full response body")
}

// ==================== HELPER FUNCTIONS ====================

// TestGenerateNewUUID helper to generate a new migration UUID if needed
func TestGenerateNewUUID(t *testing.T) {
	newUUID := uuid.New()
	t.Logf("New Migration UUID: %s", newUUID.String())
	t.Log("Copy this UUID to testMigrationUUID variable if you want to use it")
}

// ==================== TEST RESPONSE STATUS CODES ====================

// TestYBMResponseDetails tests and displays detailed response information
func TestYBAeonResponseDetails(t *testing.T) {
	log.SetLevel(log.DebugLevel) // Enable debug to see full responses

	config := &YBAeonConfig{
		Domain:    testDomain,
		AccountID: testAccountID,
		ProjectID: testProjectID,
		ClusterID: testClusterID,
		APIKey:    testAPIKey,
	}

	ybaeon := New(testExportDir, config)
	if err := ybaeon.Init(); err != nil {
		t.Fatalf("Failed to initialize YBM: %v", err)
	}

	t.Log("==================== Testing Metadata Endpoint ====================")

	// Test metadata POST
	t.Log("\n📤 Sending POST request to /voyager/metadata")
	t.Logf("   Migration UUID: %s", testMetadataPayload.MigrationUUID)
	t.Logf("   Phase: %d, Sequence: %d, Status: %s",
		testMetadataPayload.MigrationPhase,
		testMetadataPayload.InvocationSequence,
		testMetadataPayload.Status)

	err := ybaeon.sendMigrationEvent(testMetadataPayload)
	if err != nil {
		t.Logf("❌ Response: ERROR")
		t.Logf("   Error Message: %v", err)

		// Parse error to determine status code
		errStr := err.Error()
		if contains(errStr, "400") {
			t.Log("   Status Code: 400 (Bad Request)")
			t.Log("   Possible causes: Duplicate entry, validation error")
		} else if contains(errStr, "401") {
			t.Log("   Status Code: 401 (Unauthorized)")
			t.Log("   Possible causes: Invalid API key")
		} else if contains(errStr, "403") {
			t.Log("   Status Code: 403 (Forbidden)")
			t.Log("   Possible causes: API key lacks permissions")
		} else if contains(errStr, "404") {
			t.Log("   Status Code: 404 (Not Found)")
			t.Log("   Possible causes: Invalid endpoint or resource")
		}
	} else {
		t.Log("✅ Response: SUCCESS")
		t.Log("   Status Code: 200 or 201")
		t.Log("   Description: Metadata created/updated successfully")
		t.Log("   Note: Check debug logs above for full response body")
	}

	t.Log("\n==================== Testing Table Metrics Endpoint ====================")

	// Test table metrics PUT
	t.Log("\n📤 Sending PUT request to /voyager/table-metrics")
	t.Logf("   Table: %s, Schema: %s", testTableMetrics.TableName, testTableMetrics.SchemaName)
	t.Logf("   Phase: %d, Status: %d, Rows: %d/%d",
		testTableMetrics.MigrationPhase,
		testTableMetrics.Status,
		testTableMetrics.CountLiveRows,
		testTableMetrics.CountTotalRows)

	err = ybaeon.sendTableMetrics([]cp.TableMetrics{testTableMetrics})
	if err != nil {
		t.Logf("❌ Response: ERROR")
		t.Logf("   Error Message: %v", err)

		errStr := err.Error()
		if contains(errStr, "400") {
			t.Log("   Status Code: 400 (Bad Request)")
			t.Log("   Possible causes: Validation error, invalid data")
		} else if contains(errStr, "401") {
			t.Log("   Status Code: 401 (Unauthorized)")
		} else if contains(errStr, "404") {
			t.Log("   Status Code: 404 (Not Found)")
		}
	} else {
		t.Log("✅ Response: SUCCESS")
		t.Log("   Status Code: 200 or 201")
		t.Log("   Description: Table metrics upserted successfully")
		t.Log("   Note: Check debug logs above for full response body")
	}

	t.Log("\n==================== Response Status Code Reference ====================")
	t.Log("200/201 - Success: Request processed successfully")
	t.Log("400 - Bad Request: Duplicate entry or validation error")
	t.Log("401 - Unauthorized: Invalid API key")
	t.Log("403 - Forbidden: API key lacks required permissions")
	t.Log("404 - Not Found: Endpoint or resource doesn't exist")
	t.Log("429 - Rate Limited: Too many requests")
	t.Log("500-504 - Server Error: YBM backend issue")
}

// TestYBMGetMaxSequence tests the GET /voyager/migrations endpoint for fetching max sequence
func TestYBAeonGetMaxSequence(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	config := &YBAeonConfig{
		Domain:    testDomain,
		AccountID: testAccountID,
		ProjectID: testProjectID,
		ClusterID: testClusterID,
		APIKey:    testAPIKey,
	}

	ybaeon := New(testExportDir, config)
	if err := ybaeon.Init(); err != nil {
		t.Fatalf("Failed to initialize YBM: %v", err)
	}

	// Test configuration
	testPhase := 4 // ASSESS phase
	migrationUUID := testMigrationUUID

	t.Logf("Testing GET /voyager/migrations endpoint")
	t.Logf("Migration UUID: %s", migrationUUID.String())
	t.Logf("Migration Phase: %d", testPhase)
	t.Log("")

	// Call the actual function
	maxSeq, err := ybaeon.getMaxSequenceFromAPI(migrationUUID, testPhase)

	if err != nil {
		t.Logf("❌ Failed to get max sequence")
		t.Logf("Error: %v", err)

		// Parse error to show response code
		errStr := err.Error()
		if contains(errStr, "400") {
			t.Log("Response Code: 400 (Bad Request - check parameters)")
		} else if contains(errStr, "401") {
			t.Log("Response Code: 401 (Unauthorized - check API key)")
		} else if contains(errStr, "404") {
			t.Log("Response Code: 404 (Not Found - no migrations for this phase yet)")
			t.Log("This is expected if you haven't run any migrations for this phase")
		} else {
			t.Logf("Error: %v", err)
		}

		// Don't fail the test - 404 is expected for first run
		if contains(errStr, "404") {
			t.Log("✅ Test passed: 404 is expected for first run")
			return
		}
		t.Fatalf("Test failed due to API error")
	}

	t.Log("✅ Successfully fetched max sequence from YBM")
	t.Logf("Max Invocation Sequence: %d", maxSeq)
	t.Log("Response Code: 200 (Success)")
	t.Log("")
	t.Log("Next sequence will be: ", maxSeq+1)
	t.Log("Check debug logs above for full API response")

	t.Log("\n==================== Interpretation ====================")
	if maxSeq == 0 {
		t.Log("Max sequence = 0 means:")
		t.Log("  - Either no prior migrations exist for this phase")
		t.Log("  - Or the endpoint returned empty data array")
		t.Log("  - Next migration will use sequence = 1")
	} else {
		t.Logf("Max sequence = %d means:", maxSeq)
		t.Logf("  - There have been %d previous invocations of this phase", maxSeq)
		t.Logf("  - Next migration will use sequence = %d", maxSeq+1)
	}
}

// TestYBMGetMaxSequenceMultiplePhases tests max sequence for different phases
func TestYBAeonGetMaxSequenceMultiplePhases(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	config := &YBAeonConfig{
		Domain:    testDomain,
		AccountID: testAccountID,
		ProjectID: testProjectID,
		ClusterID: testClusterID,
		APIKey:    testAPIKey,
	}

	ybaeon := New(testExportDir, config)
	if err := ybaeon.Init(); err != nil {
		t.Fatalf("Failed to initialize YBM: %v", err)
	}

	migrationUUID := testMigrationUUID

	// Test multiple phases
	phases := map[int]string{
		1: "ASSESS",
		2: "EXPORT_SCHEMA",
		3: "ANALYZE_SCHEMA",
		4: "EXPORT_DATA",
		5: "IMPORT_SCHEMA",
		6: "IMPORT_DATA",
	}

	t.Logf("Testing max sequence for multiple phases")
	t.Logf("Migration UUID: %s\n", migrationUUID.String())

	for phase, phaseName := range phases {
		t.Logf("--- Testing Phase %d (%s) ---", phase, phaseName)

		maxSeq, err := ybaeon.getMaxSequenceFromAPI(migrationUUID, phase)

		if err != nil {
			errStr := err.Error()
			if contains(errStr, "404") {
				t.Logf("Phase %d: No data yet (404) - next sequence will be 1", phase)
			} else {
				t.Logf("Phase %d: Error - %v", phase, err)
			}
		} else {
			t.Logf("Phase %d: Max sequence = %d, next = %d", phase, maxSeq, maxSeq+1)
		}
		t.Log("")
	}

	t.Log("✅ Test completed - check results above")
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && len(substr) > 0 &&
		(s == substr || (len(s) > len(substr) &&
			(findSubstring(s, substr) >= 0)))
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
