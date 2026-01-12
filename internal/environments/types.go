package environments

import (
	"time"
)

type EnvironmentType string

const (
	TypeEphemeral  EnvironmentType = "ephemeral"  // 8 hour TTL
	TypeDevSandbox EnvironmentType = "sandbox"    // Long-lived dev environment
)

type EnvironmentStatus string

const (
	StatusPending  EnvironmentStatus = "pending"
	StatusCreating EnvironmentStatus = "creating"
	StatusReady    EnvironmentStatus = "ready"
	StatusExpired  EnvironmentStatus = "expired"
	StatusDeleting EnvironmentStatus = "deleting"
	StatusDeleted  EnvironmentStatus = "deleted"
	StatusFailed   EnvironmentStatus = "failed"
)

type Environment struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Owner       string            `json:"owner"`       // email or username
	Type        EnvironmentType   `json:"type"`
	Status      EnvironmentStatus `json:"status"`

	// Timestamps
	CreatedAt   time.Time         `json:"createdAt"`
	ExpiresAt   time.Time         `json:"expiresAt,omitempty"`
	DeletedAt   *time.Time        `json:"deletedAt,omitempty"`

	// Resource info
	Namespace   string            `json:"namespace"`
	DatabaseSchema string         `json:"databaseSchema"`
	RedisPrefix string            `json:"redisPrefix,omitempty"`
	MQTTPrefix  string            `json:"mqttPrefix,omitempty"`

	// Access info
	URL         string            `json:"url"`
	InternalURL string            `json:"internalUrl"`

	// Branch/commit being tested
	Branch      string            `json:"branch,omitempty"`
	Commit      string            `json:"commit,omitempty"`

	// Error info if failed
	Error       string            `json:"error,omitempty"`
}

type CreateEnvironmentRequest struct {
	Name   string          `json:"name"`
	Owner  string          `json:"owner"`
	Type   EnvironmentType `json:"type"`
	Branch string          `json:"branch,omitempty"`
	TTLHours int           `json:"ttlHours,omitempty"` // Override default TTL
}

type ListEnvironmentsOptions struct {
	Owner  string
	Status EnvironmentStatus
	Type   EnvironmentType
}
