package database

import (
	"math/rand"
	"time"

	"github.com/testkube/dashboard/internal/testkube"
)

type MockDatabase struct {
	executions []testkube.Execution
	testCases  []TestCase
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		executions: []testkube.Execution{},
		testCases:  []TestCase{},
	}
}

func (db *MockDatabase) InsertExecution(exec testkube.Execution) error {
	db.executions = append(db.executions, exec)
	return nil
}

func (db *MockDatabase) InsertTestCase(tc TestCase) error {
	db.testCases = append(db.testCases, tc)
	return nil
}

func (db *MockDatabase) InsertK6Metric(metric K6MetricRecord) error {
	return nil
}

func (db *MockDatabase) GetTrends(days int) (*TrendData, error) {
	return &TrendData{
		CurrentPassRate: 85.5,
		PassRateChange:  "+2.1%",
		AvgDuration:     120 * time.Second,
		DurationChange:  "-5%",
	}, nil
}

func (db *MockDatabase) GetWorkflowMetrics(workflow string, days int) ([]DataPoint, error) {
	// Generate dummy data
	var points []DataPoint
	now := time.Now()
	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -i)
		points = append(points, DataPoint{
			Date:        date,
			PassRate:    80 + rand.Float64()*20,
			AvgDuration: 100 + rand.Float64()*50,
			P95Duration: 150 + rand.Float64()*50,
			Count:       10 + rand.Intn(10),
		})
	}
	return points, nil
}

func (db *MockDatabase) GetPassRateTrend(workflow string, days int) ([]DataPoint, error) {
	return db.GetWorkflowMetrics(workflow, days)
}

func (db *MockDatabase) GetDurationTrend(workflow string, days int) ([]DataPoint, error) {
	return db.GetWorkflowMetrics(workflow, days)
}

func (db *MockDatabase) GetFlakyTests(threshold float64) ([]FlakyTest, error) {
	return []FlakyTest{
		{TestName: "Checkout Process", FlakyScore: 0.45, LastFailure: time.Now().Add(-2 * time.Hour)},
		{TestName: "Login with OAuth", FlakyScore: 0.32, LastFailure: time.Now().Add(-5 * time.Hour)},
	}, nil
}

func (db *MockDatabase) GetExecutionMetrics(executionID string) ([]TestCase, error) {
	// Return dummy test cases for an execution
	return []TestCase{
		{TestName: "Login Page Loads", Status: "passed", DurationMs: 1200},
		{TestName: "Submit Form", Status: "failed", DurationMs: 5000, ErrorMessage: "Timeout waiting for selector"},
		{TestName: "Logout", Status: "passed", DurationMs: 800},
	}, nil
}

func (db *MockDatabase) GetK6Metrics(executionID string) ([]K6MetricRecord, error) {
	return []K6MetricRecord{}, nil
}
