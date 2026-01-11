package database

import (
	"time"

	"github.com/testkube/dashboard/internal/testkube"
)

type TrendData struct {
	CurrentPassRate float64
	PassRateChange  string // e.g. "+5.2%"
	AvgDuration     time.Duration
	DurationChange  string // e.g. "-12%"
}

type DataPoint struct {
	Date        time.Time
	PassRate    float64
	AvgDuration float64 // in seconds or ms
	P95Duration float64
	Count       int
}

type FlakyTest struct {
	TestName    string
	TotalRuns   int
	FailedRuns  int
	PassedRuns  int
	FlakyScore  float64
	LastFailure time.Time
}

type TestCase struct {
	ExecutionID  string
	TestName     string
	FilePath     string
	Status       string
	DurationMs   int
	ErrorMessage string
	RetryCount   int
}

type K6MetricRecord struct {
	ExecutionID string
	MetricName  string
	MetricType  string
	MinValue    float64
	MaxValue    float64
	AvgValue    float64
	P95Value    float64
	P99Value    float64
}

type Database interface {
	InsertExecution(exec testkube.Execution) error
	InsertTestCase(tc TestCase) error
	InsertK6Metric(metric K6MetricRecord) error

	GetTrends(days int) (*TrendData, error)
	GetWorkflowMetrics(workflow string, days int) ([]DataPoint, error)
	GetPassRateTrend(workflow string, days int) ([]DataPoint, error)
	GetDurationTrend(workflow string, days int) ([]DataPoint, error)
	GetFlakyTests(threshold float64) ([]FlakyTest, error)

	GetExecutionMetrics(executionID string) ([]TestCase, error)
	GetK6Metrics(executionID string) ([]K6MetricRecord, error)
}
