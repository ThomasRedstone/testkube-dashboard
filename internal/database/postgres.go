package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/testkube/dashboard/internal/testkube"
)

type PostgresDatabase struct {
	db *sql.DB
}

func NewPostgresDatabase(dsn string) (*PostgresDatabase, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	pgDb := &PostgresDatabase{db: db}
	if err := pgDb.InitSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return pgDb, nil
}

func (d *PostgresDatabase) InitSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS test_executions (
			id TEXT PRIMARY KEY,
			name TEXT,
			workflow_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TIMESTAMP NOT NULL,
			finished_at TIMESTAMP,
			duration_ms INTEGER,
			branch TEXT,
			labels JSONB
		);`,
		`CREATE TABLE IF NOT EXISTS test_cases (
			id SERIAL PRIMARY KEY,
			execution_id TEXT REFERENCES test_executions(id) ON DELETE CASCADE,
			test_name TEXT NOT NULL,
			file_path TEXT,
			status TEXT NOT NULL,
			duration_ms INTEGER,
			error_message TEXT,
			retry_count INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(execution_id, test_name)
		);`,
		`CREATE TABLE IF NOT EXISTS k6_metrics (
			id SERIAL PRIMARY KEY,
			execution_id TEXT REFERENCES test_executions(id) ON DELETE CASCADE,
			metric_name TEXT NOT NULL,
			metric_type TEXT,
			min_value FLOAT,
			max_value FLOAT,
			avg_value FLOAT,
			p95_value FLOAT,
			p99_value FLOAT,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS flaky_tests (
			test_name TEXT PRIMARY KEY,
			total_runs INTEGER DEFAULT 0,
			failed_runs INTEGER DEFAULT 0,
			passed_runs INTEGER DEFAULT 0,
			flaky_score FLOAT,
			last_failure TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_test_cases_name ON test_cases(test_name);`,
		`CREATE INDEX IF NOT EXISTS idx_test_cases_status ON test_cases(status, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_executions_workflow ON test_executions(workflow_name, started_at DESC);`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

func (d *PostgresDatabase) InsertExecution(exec testkube.Execution) error {
	var durationMs int64
	if exec.Duration > 0 {
		durationMs = exec.Duration.Milliseconds()
	}

	// For now, ignoring labels JSONB as we don't use it yet
	_, err := d.db.Exec(`
		INSERT INTO test_executions (id, name, workflow_name, status, started_at, finished_at, duration_ms, branch)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			finished_at = EXCLUDED.finished_at,
			duration_ms = EXCLUDED.duration_ms
	`, exec.ID, exec.Name, exec.WorkflowName, exec.Status, exec.StartTime, exec.EndTime, durationMs, exec.Branch)
	return err
}

func (d *PostgresDatabase) InsertTestCase(tc TestCase) error {
	_, err := d.db.Exec(`
		INSERT INTO test_cases (execution_id, test_name, file_path, status, duration_ms, error_message, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (execution_id, test_name) DO NOTHING
	`, tc.ExecutionID, tc.TestName, tc.FilePath, tc.Status, tc.DurationMs, tc.ErrorMessage, tc.RetryCount)
	return err
}

func (d *PostgresDatabase) InsertK6Metric(metric K6MetricRecord) error {
	_, err := d.db.Exec(`
		INSERT INTO k6_metrics (execution_id, metric_name, metric_type, min_value, max_value, avg_value, p95_value, p99_value)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, metric.ExecutionID, metric.MetricName, metric.MetricType, metric.MinValue, metric.MaxValue, metric.AvgValue, metric.P95Value, metric.P99Value)
	return err
}

func (d *PostgresDatabase) GetTrends(days int) (*TrendData, error) {
	// Simple implementation for now
	// This should be more complex calculating actual trends
	var total, passed int
	err := d.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN status = 'passed' THEN 1 ELSE 0 END)
		FROM test_executions
		WHERE started_at > NOW() - make_interval(days => $1)
	`, days).Scan(&total, &passed)

	if err != nil {
		return nil, err
	}

	var passRate float64
	if total > 0 {
		passRate = float64(passed) / float64(total)
	}

	// Get avg duration
	var avgDuration float64
	err = d.db.QueryRow(`
		SELECT COALESCE(AVG(duration_ms), 0)
		FROM test_executions
		WHERE started_at > NOW() - make_interval(days => $1) AND duration_ms IS NOT NULL
	`, days).Scan(&avgDuration)
	if err != nil {
		return nil, err
	}

	return &TrendData{
		CurrentPassRate: passRate,
		PassRateChange:  "0%", // TODO: Calculate change
		AvgDuration:     time.Duration(avgDuration) * time.Millisecond,
		DurationChange:  "0%", // TODO: Calculate change
	}, nil
}

func (d *PostgresDatabase) GetWorkflowMetrics(workflow string, days int) ([]DataPoint, error) {
	rows, err := d.db.Query(`
		SELECT
			DATE(started_at) as day,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'passed' THEN 1 ELSE 0 END) as passed,
			AVG(duration_ms) as avg_duration
		FROM test_executions
		WHERE workflow_name = $1 AND started_at > NOW() - make_interval(days => $2)
		GROUP BY DATE(started_at)
		ORDER BY day ASC
	`, workflow, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []DataPoint
	for rows.Next() {
		var day time.Time
		var total, passed int
		var avgDuration sql.NullFloat64

		if err := rows.Scan(&day, &total, &passed, &avgDuration); err != nil {
			return nil, err
		}

		passRate := 0.0
		if total > 0 {
			passRate = float64(passed) / float64(total) * 100
		}

		points = append(points, DataPoint{
			Date:        day,
			PassRate:    passRate,
			AvgDuration: avgDuration.Float64,
			Count:       total,
		})
	}
	return points, nil
}

func (d *PostgresDatabase) GetPassRateTrend(workflow string, days int) ([]DataPoint, error) {
	return d.GetWorkflowMetrics(workflow, days)
}

func (d *PostgresDatabase) GetDurationTrend(workflow string, days int) ([]DataPoint, error) {
	return d.GetWorkflowMetrics(workflow, days)
}

func (d *PostgresDatabase) GetFlakyTests(threshold float64) ([]FlakyTest, error) {
	rows, err := d.db.Query(`
		SELECT test_name, total_runs, failed_runs, passed_runs, flaky_score, last_failure
		FROM flaky_tests
		WHERE flaky_score >= $1
		ORDER BY flaky_score DESC, last_failure DESC
		LIMIT 20
	`, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []FlakyTest
	for rows.Next() {
		var t FlakyTest
		if err := rows.Scan(&t.TestName, &t.TotalRuns, &t.FailedRuns, &t.PassedRuns, &t.FlakyScore, &t.LastFailure); err != nil {
			return nil, err
		}
		tests = append(tests, t)
	}
	return tests, nil
}

func (d *PostgresDatabase) GetExecutionMetrics(executionID string) ([]TestCase, error) {
	rows, err := d.db.Query(`
		SELECT test_name, status, duration_ms, error_message
		FROM test_cases
		WHERE execution_id = $1
		ORDER BY test_name
	`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []TestCase
	for rows.Next() {
		var t TestCase
		var durationMs sql.NullInt64
		var errorMessage sql.NullString
		if err := rows.Scan(&t.TestName, &t.Status, &durationMs, &errorMessage); err != nil {
			return nil, err
		}
		t.DurationMs = int(durationMs.Int64)
		t.ErrorMessage = errorMessage.String
		tests = append(tests, t)
	}
	return tests, nil
}

func (d *PostgresDatabase) GetK6Metrics(executionID string) ([]K6MetricRecord, error) {
	rows, err := d.db.Query(`
		SELECT metric_name, metric_type, min_value, max_value, avg_value, p95_value, p99_value
		FROM k6_metrics
		WHERE execution_id = $1
	`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []K6MetricRecord
	for rows.Next() {
		var m K6MetricRecord
		if err := rows.Scan(&m.MetricName, &m.MetricType, &m.MinValue, &m.MaxValue, &m.AvgValue, &m.P95Value, &m.P99Value); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}
