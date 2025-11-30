// Package users provides user and permission types for GoSQLPage v2.1.
package users

import (
	"time"
)

// UserType represents the type of user.
type UserType string

const (
	UserTypeHuman  UserType = "human"
	UserTypeBot    UserType = "bot"
	UserTypeSystem UserType = "system"
)

// UserStatus represents the user's status.
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"
)

// User represents a system user.
type User struct {
	ID           string     `json:"id"`
	Type         UserType   `json:"type"`
	Username     string     `json:"username"`
	Email        string     `json:"email,omitempty"`
	PasswordHash string     `json:"-"` // never expose in JSON
	Config       string     `json:"config,omitempty"` // JSON config
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
	Status       UserStatus `json:"status"`
}

// Scope represents the permission scope.
type Scope string

const (
	ScopeGlobal   Scope = "global"
	ScopeTenant   Scope = "tenant"
	ScopeDocument Scope = "document"
	ScopeBlock    Scope = "block"
	ScopeType     Scope = "type"
)

// Action represents a permission action.
type Action string

const (
	ActionRead    Action = "read"
	ActionEdit    Action = "edit"
	ActionDelete  Action = "delete"
	ActionPublish Action = "publish"
	ActionMerge   Action = "merge"
	ActionAdmin   Action = "admin"
)

// Permission represents a permission grant.
type Permission struct {
	ID        int64      `json:"id"`
	UserID    string     `json:"user_id"` // or "*" for all users
	Scope     Scope      `json:"scope"`
	ScopeID   *string    `json:"scope_id,omitempty"`
	Action    Action     `json:"action"`
	Granted   bool       `json:"granted"`
	GrantedBy string     `json:"granted_by"`
	GrantedAt time.Time  `json:"granted_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// APIKey represents an API key for authentication.
type APIKey struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	KeyHash   string     `json:"-"` // never expose
	Scopes    []string   `json:"scopes,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// HTTPSession represents an HTTP session.
type HTTPSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// PermissionCheck represents a permission check request.
type PermissionCheck struct {
	UserID  string  `json:"user_id"`
	Action  Action  `json:"action"`
	Scope   Scope   `json:"scope"`
	ScopeID *string `json:"scope_id,omitempty"`
}

// PermissionResult represents the result of a permission check.
type PermissionResult struct {
	Allowed  bool        `json:"allowed"`
	Reason   string      `json:"reason,omitempty"`
	MatchedRule *Permission `json:"matched_rule,omitempty"`
}

// BotConfig represents configuration for a bot user.
type BotConfig struct {
	Model          string   `json:"model,omitempty"`
	RateLimit      int      `json:"rate_limit,omitempty"`
	AllowedTypes   []string `json:"allowed_types,omitempty"`
	ContextWindow  int      `json:"context_window,omitempty"`
	MaxTokens      int      `json:"max_tokens,omitempty"`
}

// UserConfig represents user preferences.
type UserConfig struct {
	Theme      string `json:"theme,omitempty"`
	Language   string `json:"language,omitempty"`
	Timezone   string `json:"timezone,omitempty"`
	EditorMode string `json:"editor_mode,omitempty"`
}
