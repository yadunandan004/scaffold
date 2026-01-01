package auth

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a refresh token in the database
type RefreshToken struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID         uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	TokenHash      string    `json:"-" gorm:"type:varchar(255);unique;not null"` // Never expose the hash
	ClientDeviceID string    `json:"client_device_id" gorm:"type:varchar(255);not null"`

	// Device metadata
	IPAddress *string `json:"ip_address" gorm:"type:inet"`
	UserAgent *string `json:"user_agent" gorm:"type:text"`

	// Timestamps
	ExpiresAt  time.Time  `json:"expires_at" gorm:"not null"`
	CreatedAt  time.Time  `json:"created_at" gorm:"default:CURRENT_TIMESTAMP"`
	LastUsedAt time.Time  `json:"last_used_at" gorm:"default:CURRENT_TIMESTAMP"`
	RevokedAt  *time.Time `json:"revoked_at"`

	// Revocation tracking
	RevokedReason *string    `json:"revoked_reason" gorm:"type:varchar(50)"`
	RevokedBy     *uuid.UUID `json:"revoked_by" gorm:"type:uuid"`

	// Usage tracking
	UseCount int `json:"use_count" gorm:"default:0"`
}

// TableName specifies the table name for RefreshToken
func (RefreshToken) TableName() string {
	return "auth.refresh_tokens"
}

// IsValid checks if the refresh token is valid (not expired and not revoked)
func (rt *RefreshToken) IsValid() bool {
	now := time.Now()
	return rt.RevokedAt == nil && rt.ExpiresAt.After(now)
}

// IsExpired checks if the refresh token has expired
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// Revoke marks the token as revoked
func (rt *RefreshToken) Revoke(reason string, revokedBy *uuid.UUID) {
	now := time.Now()
	rt.RevokedAt = &now
	rt.RevokedReason = &reason
	rt.RevokedBy = revokedBy
}

// UpdateLastUsed updates the last used timestamp and increments use count
func (rt *RefreshToken) UpdateLastUsed() {
	rt.LastUsedAt = time.Now()
	rt.UseCount++
}

// DeviceInfoData represents the structure of device information stored in JSON
type DeviceInfoData struct {
	OS          string `json:"os,omitempty"`
	OSVersion   string `json:"os_version,omitempty"`
	AppVersion  string `json:"app_version,omitempty"`
	DeviceModel string `json:"device_model,omitempty"`
	DeviceName  string `json:"device_name,omitempty"`
}

// TokenBlacklist represents a blacklisted access token
type TokenBlacklist struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TokenJTI      string     `json:"token_jti" gorm:"type:varchar(255);unique;not null"`
	UserID        *uuid.UUID `json:"user_id" gorm:"type:uuid"`
	ExpiresAt     time.Time  `json:"expires_at" gorm:"not null"`
	RevokedAt     time.Time  `json:"revoked_at" gorm:"default:CURRENT_TIMESTAMP"`
	RevokedReason string     `json:"revoked_reason" gorm:"type:varchar(50)"`
}

// TableName specifies the table name for TokenBlacklist
func (TokenBlacklist) TableName() string {
	return "auth.token_blacklist"
}

// RefreshTokenRevocationReason constants
const (
	RevocationReasonLogout         = "logout"
	RevocationReasonPasswordChange = "password_change"
	RevocationReasonExpired        = "expired"
	RevocationReasonAdmin          = "admin"
	RevocationReasonSecurity       = "security"
)
