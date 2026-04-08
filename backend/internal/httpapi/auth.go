package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authContextKey string

const currentUserKey authContextKey = "current_user"

type CurrentUser struct {
	ID    string
	Email string
}

type AuthClaims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type AuthManager struct {
	secret []byte
	ttl    time.Duration
}

func NewAuthManager(secret string, ttl time.Duration) *AuthManager {
	return &AuthManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (a *AuthManager) GenerateToken(userID, email string) (string, error) {
	now := time.Now().UTC()
	claims := AuthClaims{
		UserID: userID,
		Email:  strings.ToLower(email),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

func (a *AuthManager) ParseToken(tokenString string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AuthClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*AuthClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func HashPassword(rawPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(passwordHash, rawPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(rawPassword))
}

func withCurrentUser(ctx context.Context, user CurrentUser) context.Context {
	return context.WithValue(ctx, currentUserKey, user)
}

func currentUserFromContext(ctx context.Context) (CurrentUser, bool) {
	v, ok := ctx.Value(currentUserKey).(CurrentUser)
	if !ok {
		return CurrentUser{}, false
	}
	return v, true
}
