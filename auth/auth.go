package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"net/http"

	"github.com/google/uuid"

	"github.com/yadunandan004/scaffold/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthService struct {
	cfg        *config.AuthConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// NewAuthService creates a new AuthService with the given configuration.
// Returns an error if the RSA keys cannot be parsed.
func NewAuthService(cfg *config.AuthConfig) (*AuthService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("auth config is nil")
	}

	// Parse RSA private key
	privateKeyBlock, _ := pem.Decode([]byte(cfg.PrivateKey))
	if privateKeyBlock == nil {
		return nil, fmt.Errorf("failed to parse private key: invalid PEM block")
	}
	var privateKey *rsa.PrivateKey

	// Try PKCS8 first
	privateKeyInterface, err := x509.ParsePKCS8PrivateKey(privateKeyBlock.Bytes)
	if err == nil {
		rsaKey, ok := privateKeyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("failed to parse private key: key is not RSA type")
		}
		privateKey = rsaKey
	} else {
		// Try PKCS1 format (for RSA)
		parsedKey, pkcs1Err := x509.ParsePKCS1PrivateKey(privateKeyBlock.Bytes)
		if pkcs1Err != nil {
			return nil, fmt.Errorf("failed to parse private key: not PKCS8 (%v) or PKCS1 (%v)", err, pkcs1Err)
		}
		privateKey = parsedKey
	}

	// Parse RSA public key
	publicKeyBlock, _ := pem.Decode([]byte(cfg.PublicKey))
	if publicKeyBlock == nil {
		return nil, fmt.Errorf("failed to parse public key: invalid PEM block")
	}
	publicKeyInterface, err := x509.ParsePKIXPublicKey(publicKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to parse public key: key is not RSA type")
	}

	return &AuthService{
		cfg:        cfg,
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

func (a *AuthService) ValidateToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(a.cfg.PublicKey))
		if err != nil {
			return nil, err
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

type UserClaims struct {
	UserID         uuid.UUID `json:"user_id"`
	Email          string    `json:"email"`
	ClientDeviceID string    `json:"client_device_id"`
	jwt.RegisteredClaims
}

func (a *AuthService) HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user_claims", claims)
		c.Next()
	}
}

func (a *AuthService) GRPCInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for reflection and health check
		if info.FullMethod == "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo" ||
			info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" ||
			info.FullMethod == "/grpc.health.v1.Health/Check" ||
			info.FullMethod == "/grpc.health.v1.Health/Watch" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization token")
		}

		token := tokens[0]
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token")
		}

		ctx = context.WithValue(ctx, "user_claims", claims)
		return handler(ctx, req)
	}
}

func (a *AuthService) GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Skip auth for reflection and health check
		if info.FullMethod == "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo" ||
			info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" ||
			info.FullMethod == "/grpc.health.v1.Health/Check" ||
			info.FullMethod == "/grpc.health.v1.Health/Watch" {
			return handler(srv, ss)
		}

		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			return status.Errorf(codes.Unauthenticated, "missing authorization token")
		}

		token := tokens[0]
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid token")
		}

		// Create a wrapped stream with auth request
		ctx = context.WithValue(ctx, "user_claims", claims)
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// GenerateAccessToken generates a short-lived JWT access token
func (a *AuthService) GenerateAccessToken(userID uuid.UUID, email string, clientDeviceID string) (string, error) {
	jti := uuid.New().String() // JWT ID for potential blacklisting

	claims := jwt.MapClaims{
		"user_id":          userID.String(),
		"email":            email,
		"client_device_id": clientDeviceID,
		"token_type":       "access",
		"jti":              jti,
		"exp":              time.Now().Add(time.Duration(a.cfg.AccessTokenDuration) * time.Second).Unix(),
		"iat":              time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(a.privateKey)
}

// GenerateRefreshToken generates a secure random refresh token
func (a *AuthService) GenerateRefreshToken() (string, error) {
	// Generate 32 bytes of random data
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64 URL-safe string
	token := base64.URLEncoding.EncodeToString(tokenBytes)
	return token, nil
}

// GenerateTokenPair generates both access and refresh tokens
func (a *AuthService) GenerateTokenPair(userID uuid.UUID, email string, clientDeviceID string) (accessToken string, refreshToken string, err error) {
	// Generate access token
	accessToken, err = a.GenerateAccessToken(userID, email, clientDeviceID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err = a.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// HashRefreshToken creates a SHA256 hash of the refresh token for storage
func (a *AuthService) HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// StoreRefreshToken stores the refresh token in the database
func (a *AuthService) StoreRefreshToken(db *sql.DB, userID uuid.UUID, refreshToken string, clientDeviceID string, ipAddress string, userAgent string) error {
	// Hash the token for storage
	tokenHash := a.HashRefreshToken(refreshToken)

	// Calculate expiry time
	expiresAt := time.Now().Add(time.Duration(a.cfg.RefreshTokenDuration) * time.Second)

	// Create refresh token record
	refreshTokenRecord := &RefreshToken{
		UserID:         userID,
		TokenHash:      tokenHash,
		ClientDeviceID: clientDeviceID,
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now(),
		LastUsedAt:     time.Now(),
		UseCount:       0,
	}

	// Set optional fields only if not empty
	if ipAddress != "" {
		refreshTokenRecord.IPAddress = &ipAddress
	}
	if userAgent != "" {
		refreshTokenRecord.UserAgent = &userAgent
	}

	// Delete any existing tokens for this user/device combination
	revokeQuery := `
		UPDATE refresh_tokens
		SET revoked_at = $1, revoked_reason = $2
		WHERE user_id = $3 AND client_device_id = $4 AND revoked_at IS NULL
	`
	if _, err := db.ExecContext(context.Background(), revokeQuery,
		time.Now(), RevocationReasonLogout, userID, clientDeviceID); err != nil {
		// Log but don't fail
		fmt.Printf("Failed to revoke existing tokens: %v\n", err)
	}

	// Store the new token
	if refreshTokenRecord.ID == uuid.Nil {
		refreshTokenRecord.ID = uuid.New()
	}

	insertQuery := `
		INSERT INTO refresh_tokens (
			id, user_id, token_hash, client_device_id, ip_address, user_agent,
			expires_at, created_at, last_used_at, use_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := db.ExecContext(context.Background(), insertQuery,
		refreshTokenRecord.ID, refreshTokenRecord.UserID, refreshTokenRecord.TokenHash,
		refreshTokenRecord.ClientDeviceID, refreshTokenRecord.IPAddress, refreshTokenRecord.UserAgent,
		refreshTokenRecord.ExpiresAt, refreshTokenRecord.CreatedAt,
		refreshTokenRecord.LastUsedAt, refreshTokenRecord.UseCount)
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}

// ValidateRefreshToken validates and retrieves a refresh token from the database
func (a *AuthService) ValidateRefreshToken(db *sql.DB, refreshToken string) (*RefreshToken, error) {
	// Hash the token to compare with stored hash
	tokenHash := a.HashRefreshToken(refreshToken)

	// Look up the token in the database
	query := `
		SELECT id, user_id, token_hash, client_device_id, ip_address, user_agent,
		       expires_at, created_at, last_used_at, revoked_at, revoked_reason, revoked_by, use_count
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var storedToken RefreshToken
	err := db.QueryRowContext(context.Background(), query, tokenHash).Scan(
		&storedToken.ID, &storedToken.UserID, &storedToken.TokenHash,
		&storedToken.ClientDeviceID, &storedToken.IPAddress, &storedToken.UserAgent,
		&storedToken.ExpiresAt, &storedToken.CreatedAt, &storedToken.LastUsedAt,
		&storedToken.RevokedAt, &storedToken.RevokedReason, &storedToken.RevokedBy,
		&storedToken.UseCount,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("refresh token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query refresh token: %w", err)
	}

	// Check if token is valid
	if !storedToken.IsValid() {
		if storedToken.RevokedAt != nil {
			return nil, fmt.Errorf("refresh token has been revoked")
		}
		return nil, fmt.Errorf("refresh token has expired")
	}

	// Update last used time and use count
	storedToken.UpdateLastUsed()
	updateQuery := `
		UPDATE refresh_tokens
		SET last_used_at = $1, use_count = $2
		WHERE id = $3
	`
	if _, err := db.ExecContext(context.Background(), updateQuery,
		storedToken.LastUsedAt, storedToken.UseCount, storedToken.ID); err != nil {
		// Log but don't fail
		fmt.Printf("Failed to update refresh token usage: %v\n", err)
	}

	return &storedToken, nil
}

// RevokeRefreshToken revokes a specific refresh token
func (a *AuthService) RevokeRefreshToken(db *sql.DB, tokenID uuid.UUID, reason string, revokedBy *uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1, revoked_reason = $2, revoked_by = $3
		WHERE id = $4
	`
	_, err := db.ExecContext(context.Background(), query,
		time.Now(), reason, revokedBy, tokenID)
	return err
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (a *AuthService) RevokeAllUserTokens(db *sql.DB, userID uuid.UUID, reason string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1, revoked_reason = $2
		WHERE user_id = $3 AND revoked_at IS NULL
	`
	_, err := db.ExecContext(context.Background(), query,
		time.Now(), reason, userID)
	return err
}

// wrappedServerStream wraps a ServerStream with a custom request
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped request
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
