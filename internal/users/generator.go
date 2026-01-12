package users

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type UserGenerator struct {
	db       *sql.DB
	host     string
	user     string
	password string
}

type Environment struct {
	Name        string `json:"name"`
	Schema      string `json:"schema"`
	Description string `json:"description"`
}

type GeneratedUser struct {
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	UserType    string    `json:"userType"`
	GroupName   string    `json:"groupName"`
	Environment string    `json:"environment"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CreateUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`    // If empty, will be auto-generated
	UserType    string `json:"userType"`    // admin, user, systemadmin
	GroupName   string `json:"groupName"`   // If empty, uses default test group
	Environment string `json:"environment"` // Database schema to use
}

func NewUserGenerator() (*UserGenerator, error) {
	host := os.Getenv("DATABASE_HOST")
	user := os.Getenv("DATABASE_USER")
	password := os.Getenv("DATABASE_PASSWORD")
	if password == "" {
		password = os.Getenv("MYSQL_ROOT_PASSWORD")
	}

	// Require explicit configuration - no hardcoded defaults
	if host == "" || user == "" || password == "" {
		return &UserGenerator{}, nil // Return without DB connection
	}

	// Connect without specifying a database - we'll switch schemas dynamically
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/?parseTime=true", user, password, host)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return &UserGenerator{
		db:       db,
		host:     host,
		user:     user,
		password: password,
	}, nil
}

// ListEnvironments returns available database schemas
func (g *UserGenerator) ListEnvironments() ([]Environment, error) {
	if g.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	// Get schema pattern from env, default to showing all non-system schemas
	schemaPattern := os.Getenv("DATABASE_SCHEMA_PATTERN")
	defaultSchema := os.Getenv("DATABASE_DEFAULT_SCHEMA")

	var query string
	if schemaPattern != "" {
		query = fmt.Sprintf(`
			SELECT SCHEMA_NAME
			FROM information_schema.SCHEMATA
			WHERE SCHEMA_NAME LIKE '%s'
			ORDER BY SCHEMA_NAME
		`, schemaPattern)
	} else {
		query = `
			SELECT SCHEMA_NAME
			FROM information_schema.SCHEMATA
			WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
			ORDER BY SCHEMA_NAME
		`
	}

	rows, err := g.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var envs []Environment
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			continue
		}

		env := Environment{
			Schema: schema,
			Name:   schema,
		}

		// Mark default schema
		if schema == defaultSchema {
			env.Name = fmt.Sprintf("Default (%s)", schema)
			env.Description = "Main environment"
		} else if strings.HasPrefix(schema, "env_") {
			env.Name = strings.TrimPrefix(schema, "env_")
			env.Description = "Ephemeral environment"
		} else {
			env.Description = "Database schema"
		}

		envs = append(envs, env)
	}

	return envs, nil
}

func (g *UserGenerator) CreateUser(req CreateUserRequest) (*GeneratedUser, error) {
	if g.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	// Get defaults from environment
	defaultSchema := os.Getenv("DATABASE_DEFAULT_SCHEMA")
	emailDomain := os.Getenv("TEST_USER_EMAIL_DOMAIN")
	if emailDomain == "" {
		emailDomain = "test.local"
	}

	schema := req.Environment
	if schema == "" {
		schema = defaultSchema
	}
	if schema == "" {
		return nil, fmt.Errorf("no environment specified and DATABASE_DEFAULT_SCHEMA not set")
	}

	// Generate password if not provided
	password := req.Password
	if password == "" {
		password = generatePassword(12)
	}

	// Generate salt and hash
	salt := generateSalt()
	hash := hashPassword(password, salt)

	// Default values
	username := req.Username
	if username == "" {
		username = fmt.Sprintf("testuser_%d", time.Now().Unix())
	}

	email := req.Email
	if email == "" {
		email = fmt.Sprintf("%s@%s", username, emailDomain)
	}

	userType := req.UserType
	if userType == "" {
		userType = "user"
	}

	groupName := req.GroupName
	if groupName == "" {
		groupName = "Test Users"
	}

	// Ensure group exists
	groupID, err := g.ensureGroup(schema, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure group: %w", err)
	}

	// Insert user using the specified schema
	query := fmt.Sprintf(`
		INSERT INTO %s.users (user_name, user_type, user_group_id, user_email, user_password, user_salt, user_login_failed_attempts, user_disabled)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
		ON DUPLICATE KEY UPDATE
			user_password = VALUES(user_password),
			user_salt = VALUES(user_salt),
			user_login_failed_attempts = 0,
			user_disabled = 0
	`, schema)

	_, err = g.db.Exec(query, username, userType, groupID, email, hash, salt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &GeneratedUser{
		Username:    username,
		Email:       email,
		Password:    password,
		UserType:    userType,
		GroupName:   groupName,
		Environment: schema,
		CreatedAt:   time.Now(),
	}, nil
}

func (g *UserGenerator) ensureGroup(schema, groupName string) (int64, error) {
	// Try to get existing group
	var groupID int64
	query := fmt.Sprintf("SELECT user_group_id FROM %s.user_groups WHERE user_group_name = ?", schema)
	err := g.db.QueryRow(query, groupName).Scan(&groupID)
	if err == nil {
		return groupID, nil
	}

	// Create new group
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s.user_groups (user_group_name, user_group_description, user_group_status)
		VALUES (?, ?, 'active')
	`, schema)
	result, err := g.db.Exec(insertQuery, groupName, "Auto-generated test group")
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (g *UserGenerator) ListRecentUsers(limit int, environment string) ([]GeneratedUser, error) {
	if g.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	if limit <= 0 {
		limit = 20
	}

	schema := environment
	if schema == "" {
		schema = os.Getenv("DATABASE_DEFAULT_SCHEMA")
	}
	if schema == "" {
		return nil, fmt.Errorf("no environment specified and DATABASE_DEFAULT_SCHEMA not set")
	}

	query := fmt.Sprintf(`
		SELECT u.user_name, u.user_email, u.user_type, g.user_group_name, u.created_at
		FROM %s.users u
		LEFT JOIN %s.user_groups g ON u.user_group_id = g.user_group_id
		WHERE u.user_email LIKE '%%test%%' OR u.user_email LIKE '%%texecom.local'
		ORDER BY u.created_at DESC
		LIMIT ?
	`, schema, schema)

	rows, err := g.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []GeneratedUser
	for rows.Next() {
		var u GeneratedUser
		var createdAt sql.NullTime
		var groupName sql.NullString
		if err := rows.Scan(&u.Username, &u.Email, &u.UserType, &groupName, &createdAt); err != nil {
			continue
		}
		if createdAt.Valid {
			u.CreatedAt = createdAt.Time
		}
		if groupName.Valid {
			u.GroupName = groupName.String
		}
		u.Environment = schema
		users = append(users, u)
	}

	return users, nil
}

func (g *UserGenerator) DeleteUser(username, environment string) error {
	if g.db == nil {
		return fmt.Errorf("database not configured")
	}

	schema := environment
	if schema == "" {
		schema = os.Getenv("DATABASE_DEFAULT_SCHEMA")
	}
	if schema == "" {
		return fmt.Errorf("no environment specified and DATABASE_DEFAULT_SCHEMA not set")
	}

	query := fmt.Sprintf("DELETE FROM %s.users WHERE user_name = ?", schema)
	_, err := g.db.Exec(query, username)
	return err
}

// generatePassword creates a random password
func generatePassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	bytes := make([]byte, length)
	rand.Read(bytes)
	for i := range bytes {
		bytes[i] = chars[int(bytes[i])%len(chars)]
	}
	return string(bytes)
}

// generateSalt creates a random salt for password hashing
func generateSalt() string {
	bytes := make([]byte, 30)
	rand.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)
}

// hashPassword creates a SHA256 hash of password+salt (matching texecom-cloud's scheme)
func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(password + salt))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
