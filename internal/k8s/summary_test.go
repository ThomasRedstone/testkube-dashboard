package k8s

import (
	"context"
	"testing"
)

func TestGetDashboardSummary(t *testing.T) {
	// Create a service with known data
	svc := NewMockK8sService()

	// Add some execution data to the mock to test calculations
	// The mock currently has hardcoded data, let's ensure we can test against it
	// or modify the mock to allow injecting data.
	// For this test, relying on the hardcoded mock data is brittle if we change it later.
	// A better approach for the test is to make the mock configurable or inspect what's there.
	// But `NewMockK8sService` initializes with data.

	// Let's rely on the current Mock implementation which we know returns:
	// 3 tests.
	// Executions for "api-sanity-check": 1 passed, 1 failed.

	// We need to implement the logic in the mock to aggregate this.
	// So we expect:
	// TotalTests: 3
	// TotalExecutions: 2 (from api-sanity-check) + others?
	// actually ListExecutions in mock only returns for the specific test requested or we might need to change how mock stores data.

	ctx := context.Background()
	summary, err := svc.GetDashboardSummary(ctx, "testkube")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalTests != 3 {
		t.Errorf("expected 3 total tests, got %d", summary.TotalTests)
	}

	// Based on current mock data in mock_client.go:
	// api-sanity-check: exec-1 (passed), exec-2 (failed)
	// others: (empty in current ListTests but ListExecutions is dynamic based on test name input?)
	//
	// Wait, looking at `mock_client.go`:
	// `ListExecutions` returns the SAME dummy data regardless of test name input?
	// No, let's check the code.
	/*
	func (s *MockK8sService) ListExecutions(ctx context.Context, namespace, testName string) ([]app.TestExecution, error) {
		return []app.TestExecution{
			{ID: "exec-1", TestName: testName, Status: "passed", ...},
			{ID: "exec-2", TestName: testName, Status: "failed", ...},
		}, nil
	}
	*/
	// So for EVERY test, it returns 2 executions.
	// Since there are 3 tests, TotalExecutions should be 3 * 2 = 6.
	// Passed: 3, Failed: 3.
	// PassRate: 50%.

	if summary.TotalExecutions != 6 {
		t.Errorf("expected 6 total executions, got %d", summary.TotalExecutions)
	}

	if summary.PassRate != 50.0 {
		t.Errorf("expected 50.0 pass rate, got %f", summary.PassRate)
	}

	if len(summary.RecentFailures) != 3 {
		t.Errorf("expected 3 recent failures (1 per test), got %d", len(summary.RecentFailures))
	}
}
