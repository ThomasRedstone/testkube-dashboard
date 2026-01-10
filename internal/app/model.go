package app

import "time"

// Test represents a Testkube Test
type Test struct {
	Name      string
	Namespace string
	Type      string
	Labels    map[string]string
	Created   time.Time
}

// TestExecution represents a run of a test
type TestExecution struct {
	ID        string
	TestName  string
	Status    string // passed, failed, running, queued
	StartTime time.Time
	EndTime   time.Time
}
