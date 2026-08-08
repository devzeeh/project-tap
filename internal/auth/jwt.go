package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(getEnv("JWT_SECRET", "super-secret-key")) // fallback for dev only

// getEnv retrieves the value of the environment variable named by the key, or returns the fallback value if not present.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// JWTClaims represents the custom and standard claims embedded within the JSON Web Token.
type JWTClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// GenerateTokens creates a short-lived access token (15 min) and a long-lived
// refresh token (7 days).
func GenerateTokens(userID, role string) (accessToken, refreshToken string, err error) {
	accessToken, err = signToken(userID, role, "access", AccessTokenTTL)
	if err != nil {
		return
	}
	refreshToken, err = signToken(userID, role, "refresh", RefreshTokenTTL)
	return
}

// ValidateJWT parses and validates the token string, returning the embedded
// claims on success.
func ValidateJWT(tokenString string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// internal helper

// signToken creates and signs a new JWT with the specified claims and expiration time.
func signToken(userID, role, subject string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "unicard-auth",
			Subject:   subject,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}