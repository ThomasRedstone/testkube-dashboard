package users

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type UserGenerator struct {
	db *sql.DB
}

type GeneratedUser struct {
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	UserType  string    `json:"userType"`
	GroupName string    `json:"groupName"`
	CreatedAt time.Time `json:"createdAt"`
}

type CreateUserRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`  // If empty, will be auto-generated
	UserType  string `json:"userType"`  // admin, user, systemadmin
	GroupName string `json:"groupName"` // If empty, uses default test group
}

func NewUserGenerator() (*UserGenerator, error) {
	host := os.Getenv("DATABASE_HOST")
	if host == "" {
		host = "texecom-texecom-cloud-mysql.texecom.svc.cluster.local"
	}

	user := os.Getenv("DATABASE_USER")
	if user == "" {
		user = "root"
	}

	password := os.Getenv("DATABASE_PASSWORD")
	if password == "" {
		password = os.Getenv("MYSQL_ROOT_PASSWORD")
	}

	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = "texecomcloud"
	}

	if password == "" {
		return &UserGenerator{}, nil // Return without DB connection
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", user, password, host, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return &UserGenerator{db: db}, nil
}

func (g *UserGenerator) CreateUser(req CreateUserRequest) (*GeneratedUser, error) {
	if g.db == nil {
		return nil, fmt.Errorf("database not configured")
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
		email = fmt.Sprintf("%s@test.texecom.local", username)
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
	groupID, err := g.ensureGroup(groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure group: %w", err)
	}

	// Insert user
	_, err = g.db.Exec(`
		INSERT INTO users (user_name, user_type, user_group_id, user_email, user_password, user_salt, user_login_failed_attempts, user_disabled)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
		ON DUPLICATE KEY UPDATE
			user_password = VALUES(user_password),
			user_salt = VALUES(user_salt),
			user_login_failed_attempts = 0,
			user_disabled = 0
	`, username, userType, groupID, email, hash, salt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &GeneratedUser{
		Username:  username,
		Email:     email,
		Password:  password,
		UserType:  userType,
		GroupName: groupName,
		CreatedAt: time.Now(),
	}, nil
}

func (g *UserGenerator) ensureGroup(groupName string) (int64, error) {
	// Try to get existing group
	var groupID int64
	err := g.db.QueryRow("SELECT user_group_id FROM user_groups WHERE user_group_name = ?", groupName).Scan(&groupID)
	if err == nil {
		return groupID, nil
	}

	// Create new group
	result, err := g.db.Exec(`
		INSERT INTO user_groups (user_group_name, user_group_description, user_group_status)
		VALUES (?, ?, 'active')
	`, groupName, "Auto-generated test group")
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (g *UserGenerator) ListRecentUsers(limit int) ([]GeneratedUser, error) {
	if g.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	if limit <= 0 {
		limit = 20
	}

	rows, err := g.db.Query(`
		SELECT u.user_name, u.user_email, u.user_type, g.user_group_name, u.created_at
		FROM users u
		LEFT JOIN user_groups g ON u.user_group_id = g.user_group_id
		WHERE u.user_email LIKE '%test%' OR u.user_email LIKE '%texecom.local'
		ORDER BY u.created_at DESC
		LIMIT ?
	`, limit)
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
		users = append(users, u)
	}

	return users, nil
}

func (g *UserGenerator) DeleteUser(username string) error {
	if g.db == nil {
		return fmt.Errorf("database not configured")
	}

	_, err := g.db.Exec("DELETE FROM users WHERE user_name = ?", username)
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
