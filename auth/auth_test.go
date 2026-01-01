package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yadunandan004/scaffold/config"
)

// Test RSA keys for testing
const testPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEApKZtiRFe6c5Noe+5nWUQVEoqqul1g4l0UL+jMs9vuE5+Pkf7
/FaQdCSMyIUrJcizzMBEmWfnjGwg+UdGTyu149PSBr/Qdl1H3F5V7kH5uiVmV/jw
NQG+OQyMUU2sh0o4QRZ6XWHslZeA4Hbr4eKCxaI7a/JTgphrz0gKffpBwWk8nFfl
lxUACsqGXAQZu1wwPBrqQ7Syfy7jHrv/PHcVNX0120HOL4a8Tq4wbRjZhHkM+pdt
/ovcB/OFq92Ws7aXVB8+okXGJGp4phc665TpMLhqtilNArfWHnfkT9rdTLe0LkEs
I6ShuEpopIyJRW7zs/BX/HaKlx1PBlo7sBXzZwIDAQABAoIBABBoA7AjH/myNJV3
RV/uoHLrCldfeKCJGM2sWtRcS9M/VwsDseiSA5uJwibU8Ni3UrCLVAqT35m5czZY
/iOIxSYU3EUPO0l1ardbBbr+WKZOGxBBs5sZ4lzdkuR4a0GhM/vLA+8RgTk4EH/N
ZHHCAqqIy4eqbNyC2EieSBVc03ffHgr2MZcwVTIhfwGmdupp13ornEySpfaXjDCD
kNToZef1JlgUa5RYnHSpakoZEn/wlGZNl7GP+FlTeIui86Si/YiFXrCWzVXiAq1Q
LXrwL0nAUSxSXNUHha/uR/GDL8TwyVuXIV8/OKNGaf+FN6Q1zloUgqPwgQ3MwSBL
DbhB+ykCgYEAzewKPHPwOo+y9oCa9XugbgQcV1f22HjD8JUBqPVMM6soa1aTNYk/
LLEGNELp+1kA4qoNqvgl1I9almSZiDjt9/+SEWfULTZFjrJ0StbZu0JQV5tPSVUw
DowMqLPOixlHg2fEPitE3ORAvzYul6XqPB9zeRs18ERKCTX3a3/KQYsCgYEAzLDy
jCOhUCh+PSLD8x74aG7wTJX7xPAu9t+4RZyePur5ZnDDhGeMpGHM4C5I+pct7h5V
i5nPXhhdRldRrLoOdcT/rS8lvr7/RXmUy0rNrHoO8jCMYD5n6BrKKHljhU1GyChN
nVf9Zz9bWPCIwkAwRIpnwHsgVxFTqT2bna9IGRUCgYAD9+uOlLFpf4F0bLAP0Q0b
carWKBTSwSkNhuGcTvXj/QVvZCC8JGP6SYMUGMIHnQR+Wcafp96axRR6139595bm
c59uBHE7WdNnV5sUIiXaDQIdXhneEO03Ko3H5ocxeRA+wQ4wIIdYNnHk/XdSZtkn
xXdlOxgEBFzk5oxZHwJX1wKBgCWHi/EF113LDtpGtYat9v2u2YAxP6gsIXBCNJcO
0DTZAEE4C6ELG05IYDf6RIctkM5H4Ydm/A5UiUWMXP0+X8hYBkjKjDEc89DZKd7c
KDmnZ3YgUJyU1JhJ0Sb6mrSmJoQsX46pw1xa0XTNJUX4XuEyPzObX6KXGq+9C/st
WBrBAoGAeljVyxdvyadO7yzDtGTsmUefUG2Rx3QxPUGl77tXYMfyN+szFJ/bh+0J
zQvwspA1Dr6wZiovRBVTVJjdA0l3cGpDTrnHew7jC5fQlC7GrSDeNkxpxG7hSDzF
HGDDe68MfWrRmdIgIgI8VBTDAhh5ad6bR4fWkatvY++/wLDvDew=
-----END RSA PRIVATE KEY-----`

const testPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEApKZtiRFe6c5Noe+5nWUQ
VEoqqul1g4l0UL+jMs9vuE5+Pkf7/FaQdCSMyIUrJcizzMBEmWfnjGwg+UdGTyu1
49PSBr/Qdl1H3F5V7kH5uiVmV/jwNQG+OQyMUU2sh0o4QRZ6XWHslZeA4Hbr4eKC
xaI7a/JTgphrz0gKffpBwWk8nFfllxUACsqGXAQZu1wwPBrqQ7Syfy7jHrv/PHcV
NX0120HOL4a8Tq4wbRjZhHkM+pdt/ovcB/OFq92Ws7aXVB8+okXGJGp4phc665Tp
MLhqtilNArfWHnfkT9rdTLe0LkEsI6ShuEpopIyJRW7zs/BX/HaKlx1PBlo7sBXz
ZwIDAQAB
-----END PUBLIC KEY-----`

func newTestAuthService(t *testing.T) *AuthService {
	cfg := &config.AuthConfig{
		PrivateKey:           testPrivateKey,
		PublicKey:            testPublicKey,
		AccessTokenDuration:  1800,    // 30 minutes
		RefreshTokenDuration: 7776000, // 90 days
	}
	svc, err := NewAuthService(cfg)
	require.NoError(t, err, "failed to create auth service")
	return svc
}

func TestGenerateAccessToken(t *testing.T) {
	authService := newTestAuthService(t)

	userID := uuid.New()
	email := "test@example.com"
	clientDeviceID := "test-device-123"

	// Generate access token
	token, err := authService.GenerateAccessToken(userID, email, clientDeviceID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	claims := &UserClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return authService.publicKey, nil
	})
	require.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	// Verify claims
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, clientDeviceID, claims.ClientDeviceID)
}

func TestGenerateRefreshToken(t *testing.T) {
	authService := newTestAuthService(t)

	// Generate refresh token
	token, err := authService.GenerateRefreshToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Ensure token is unique
	token2, err := authService.GenerateRefreshToken()
	require.NoError(t, err)
	assert.NotEqual(t, token, token2)
}

func TestGenerateTokenPair(t *testing.T) {
	authService := newTestAuthService(t)

	userID := uuid.New()
	email := "test@example.com"
	clientDeviceID := "test-device-123"

	// Generate token pair
	accessToken, refreshToken, err := authService.GenerateTokenPair(userID, email, clientDeviceID)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NotEqual(t, accessToken, refreshToken)
}

func TestValidateToken(t *testing.T) {
	authService := newTestAuthService(t)

	userID := uuid.New()
	email := "test@example.com"
	clientDeviceID := "test-device-123"

	// Generate access token
	token, err := authService.GenerateAccessToken(userID, email, clientDeviceID)
	require.NoError(t, err)

	// Validate the token
	claims, err := authService.ValidateToken(token)
	require.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, clientDeviceID, claims.ClientDeviceID)
}

func TestValidateToken_Invalid(t *testing.T) {
	authService := newTestAuthService(t)

	// Validate an invalid token
	claims, err := authService.ValidateToken("invalid-token")
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestHashRefreshToken(t *testing.T) {
	authService := newTestAuthService(t)

	token := "test-refresh-token"
	hash1 := authService.HashRefreshToken(token)
	hash2 := authService.HashRefreshToken(token)

	// Same token should produce same hash
	assert.Equal(t, hash1, hash2)

	// Different token should produce different hash
	hash3 := authService.HashRefreshToken("different-token")
	assert.NotEqual(t, hash1, hash3)
}

func TestRefreshTokenIsValid(t *testing.T) {
	now := time.Now()
	revokedAt := now

	tests := []struct {
		name     string
		token    RefreshToken
		expected bool
	}{
		{
			name: "valid token",
			token: RefreshToken{
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: nil,
			},
			expected: true,
		},
		{
			name: "expired token",
			token: RefreshToken{
				ExpiresAt: now.Add(-time.Hour),
				RevokedAt: nil,
			},
			expected: false,
		},
		{
			name: "revoked token",
			token: RefreshToken{
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: &revokedAt,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsValid()
			assert.Equal(t, tt.expected, result)
		})
	}
}
