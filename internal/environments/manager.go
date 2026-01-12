package environments

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	DefaultEphemeralTTL = 8 * time.Hour
	DefaultSandboxTTL   = 7 * 24 * time.Hour // 1 week
)

type Manager struct {
	environments map[string]*Environment
	mu           sync.RWMutex

	// Kubernetes client config
	namespace     string
	kubeConfig    string
	baseImage     string
	mysqlHost     string
	mysqlUser     string
	mysqlPassword string
	redisHost     string
	mqttHost      string
	baseURL       string
}

func NewManager() *Manager {
	m := &Manager{
		environments:  make(map[string]*Environment),
		namespace:     getEnvOrDefault("ENVIRONMENTS_NAMESPACE", "texecom-envs"),
		baseImage:     getEnvOrDefault("FERN_IMAGE", "534294601285.dkr.ecr.eu-west-2.amazonaws.com/develop/texecom-cloud:latest"),
		mysqlHost:     getEnvOrDefault("MYSQL_HOST", "texecom-texecom-cloud-mysql.texecom.svc.cluster.local"),
		mysqlUser:     getEnvOrDefault("MYSQL_USER", "root"),
		mysqlPassword: os.Getenv("MYSQL_ROOT_PASSWORD"),
		redisHost:     getEnvOrDefault("REDIS_HOST", "texecom-texecom-cloud-redis.texecom.svc.cluster.local"),
		mqttHost:      getEnvOrDefault("MQTT_HOST", "texecom-texecom-cloud-emqx.texecom.svc.cluster.local"),
		baseURL:       getEnvOrDefault("ENVIRONMENTS_BASE_URL", "envs.services.texecom-develop.com"),
	}

	// Start background cleanup goroutine
	go m.cleanupLoop()

	return m
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func (m *Manager) generateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (m *Manager) Create(ctx context.Context, req CreateEnvironmentRequest) (*Environment, error) {
	id := m.generateID()
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("env-%s", id)
	}
	// Sanitize name for Kubernetes
	name = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	name = strings.ReplaceAll(name, "_", "-")

	// Calculate TTL
	ttl := DefaultEphemeralTTL
	if req.Type == TypeDevSandbox {
		ttl = DefaultSandboxTTL
	}
	if req.TTLHours > 0 {
		ttl = time.Duration(req.TTLHours) * time.Hour
	}

	env := &Environment{
		ID:             id,
		Name:           name,
		Owner:          req.Owner,
		Type:           req.Type,
		Status:         StatusCreating,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(ttl),
		Namespace:      m.namespace,
		DatabaseSchema: fmt.Sprintf("texecom_env_%s", id),
		RedisPrefix:    fmt.Sprintf("env:%s:", id),
		MQTTPrefix:     fmt.Sprintf("env/%s/", id),
		Branch:         req.Branch,
		InternalURL:    fmt.Sprintf("http://%s-fern.%s.svc.cluster.local:8080", name, m.namespace),
		URL:            fmt.Sprintf("https://%s.%s", name, m.baseURL),
	}

	m.mu.Lock()
	m.environments[id] = env
	m.mu.Unlock()

	// Create resources in background
	go m.provisionEnvironment(env)

	return env, nil
}

func (m *Manager) provisionEnvironment(env *Environment) {
	log.Printf("Provisioning environment %s (%s)", env.Name, env.ID)

	// Step 1: Create database schema
	if err := m.createDatabaseSchema(env); err != nil {
		m.setError(env, fmt.Sprintf("Failed to create database: %v", err))
		return
	}

	// Step 2: Create Kubernetes resources
	if err := m.createKubernetesResources(env); err != nil {
		m.setError(env, fmt.Sprintf("Failed to create k8s resources: %v", err))
		return
	}

	// Step 3: Wait for deployment to be ready
	if err := m.waitForReady(env); err != nil {
		m.setError(env, fmt.Sprintf("Environment failed to become ready: %v", err))
		return
	}

	m.mu.Lock()
	env.Status = StatusReady
	m.mu.Unlock()

	log.Printf("Environment %s is ready at %s", env.Name, env.URL)
}

func (m *Manager) createDatabaseSchema(env *Environment) error {
	if m.mysqlPassword == "" {
		log.Printf("Warning: No MySQL password configured, skipping schema creation")
		return nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/", m.mysqlUser, m.mysqlPassword, m.mysqlHost)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	defer db.Close()

	// Create schema
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", env.DatabaseSchema))
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Clone structure from main database (simplified - in production you'd want migrations)
	// For now, assume the app handles schema creation on startup

	log.Printf("Created database schema: %s", env.DatabaseSchema)
	return nil
}

func (m *Manager) createKubernetesResources(env *Environment) error {
	// Generate Kubernetes manifests and apply them
	// Using kubectl exec for simplicity - in production use client-go

	manifest := m.generateManifest(env)

	// Write manifest to temp file and apply
	tmpFile := fmt.Sprintf("/tmp/env-%s.yaml", env.ID)
	if err := os.WriteFile(tmpFile, []byte(manifest), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// This would be replaced with proper Kubernetes client in production
	log.Printf("Kubernetes manifest generated for %s", env.Name)
	log.Printf("Apply with: kubectl apply -f %s", tmpFile)

	return nil
}

func (m *Manager) generateManifest(env *Environment) string {
	return fmt.Sprintf(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s-fern
  namespace: %s
  labels:
    app: fern
    environment: %s
    env-id: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: fern
      env-id: %s
  template:
    metadata:
      labels:
        app: fern
        env-id: %s
    spec:
      containers:
        - name: fern
          image: %s
          ports:
            - containerPort: 8080
          env:
            - name: NODE_ENV
              value: development
            - name: DATABASE_HOST
              value: %s
            - name: DATABASE_NAME
              value: %s
            - name: DATABASE_USER
              value: texecom
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: texecom-cloud-secrets
                  key: mysql-password
            - name: REDIS_HOST
              value: %s
            - name: REDIS_PREFIX
              value: "%s"
            - name: MQTT_HOST
              value: %s
            - name: MQTT_TOPIC_PREFIX
              value: "%s"
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: %s-fern
  namespace: %s
  labels:
    env-id: %s
spec:
  selector:
    app: fern
    env-id: %s
  ports:
    - port: 8080
      targetPort: 8080
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s-ingress
  namespace: %s
  labels:
    env-id: %s
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/group.name: texecom-platform
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443}]'
    alb.ingress.kubernetes.io/ssl-redirect: "443"
spec:
  rules:
    - host: %s.%s
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %s-fern
                port:
                  number: 8080
`,
		env.Name, env.Namespace, env.Name, env.ID,
		env.ID, env.ID,
		m.baseImage,
		m.mysqlHost, env.DatabaseSchema,
		m.redisHost, env.RedisPrefix,
		m.mqttHost, env.MQTTPrefix,
		env.Name, env.Namespace, env.ID, env.ID,
		env.Name, env.Namespace, env.ID,
		env.Name, m.baseURL, env.Name,
	)
}

func (m *Manager) waitForReady(env *Environment) error {
	// In production, poll Kubernetes for deployment readiness
	// For now, just wait a bit
	time.Sleep(5 * time.Second)
	return nil
}

func (m *Manager) setError(env *Environment, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	env.Status = StatusFailed
	env.Error = errMsg
	log.Printf("Environment %s failed: %s", env.Name, errMsg)
}

func (m *Manager) Get(id string) (*Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	env, ok := m.environments[id]
	if !ok {
		return nil, fmt.Errorf("environment not found: %s", id)
	}
	return env, nil
}

func (m *Manager) List(opts ListEnvironmentsOptions) []*Environment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Environment
	for _, env := range m.environments {
		if opts.Owner != "" && env.Owner != opts.Owner {
			continue
		}
		if opts.Status != "" && env.Status != opts.Status {
			continue
		}
		if opts.Type != "" && env.Type != opts.Type {
			continue
		}
		// Don't include deleted environments
		if env.Status == StatusDeleted {
			continue
		}
		result = append(result, env)
	}
	return result
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	env, ok := m.environments[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("environment not found: %s", id)
	}
	env.Status = StatusDeleting
	m.mu.Unlock()

	go m.teardownEnvironment(env)
	return nil
}

func (m *Manager) teardownEnvironment(env *Environment) {
	log.Printf("Tearing down environment %s", env.Name)

	// Delete Kubernetes resources
	// kubectl delete -l env-id=<id> --namespace=<ns>

	// Drop database schema
	if m.mysqlPassword != "" {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/", m.mysqlUser, m.mysqlPassword, m.mysqlHost)
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", env.DatabaseSchema))
			db.Close()
		}
	}

	m.mu.Lock()
	now := time.Now()
	env.Status = StatusDeleted
	env.DeletedAt = &now
	m.mu.Unlock()

	log.Printf("Environment %s deleted", env.Name)
}

func (m *Manager) Extend(id string, hours int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.environments[id]
	if !ok {
		return fmt.Errorf("environment not found: %s", id)
	}

	env.ExpiresAt = env.ExpiresAt.Add(time.Duration(hours) * time.Hour)
	log.Printf("Extended environment %s until %s", env.Name, env.ExpiresAt)
	return nil
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.checkExpired()
	}
}

func (m *Manager) checkExpired() {
	m.mu.RLock()
	var toDelete []string
	for id, env := range m.environments {
		if env.Status == StatusReady && time.Now().After(env.ExpiresAt) {
			toDelete = append(toDelete, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range toDelete {
		log.Printf("Environment %s has expired, cleaning up", id)
		m.Delete(id)
	}
}
