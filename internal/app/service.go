package app

import "context"

// K8sService defines the operations we need from Kubernetes
type K8sService interface {
	ListTests(ctx context.Context, namespace string) ([]Test, error)
	GetTest(ctx context.Context, namespace, name string) (*Test, error)
	ListExecutions(ctx context.Context, namespace, testName string) ([]TestExecution, error)
	GetExecutionLogs(ctx context.Context, namespace, executionID string) (string, error)
	GetDashboardSummary(ctx context.Context, namespace string) (*DashboardSummary, error)
}
