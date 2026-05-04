package aaa

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AAA struct {
	users    map[string]string
	tokenTTL time.Duration
	log      *slog.Logger
	secret   []byte
}

func New(tokenTTL time.Duration, log *slog.Logger) (*AAA, error) {
	user := os.Getenv("ADMIN_USER")
	password := os.Getenv("ADMIN_PASSWORD")
	if user == "" || password == "" {
		return nil, errors.New("ADMIN_USER and ADMIN_PASSWORD must be set")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Warn("JWT_SECRET not set, using default (insecure for production)")
		secret = "default-insecure-secret"
	}
	return &AAA{
		users:    map[string]string{user: password},
		tokenTTL: tokenTTL,
		log:      log,
		secret:   []byte(secret),
	}, nil
}

func (a *AAA) Login(name, password string) (string, error) {
	expectedPass, exists := a.users[name]
	if !exists || expectedPass != password {
		return "", errors.New("invalid credentials")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "superuser",
		"exp": time.Now().Add(a.tokenTTL).Unix(),
	})
	return token.SignedString(a.secret)
}

func (a *AAA) Verify(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["sub"] != "superuser" {
		return errors.New("not a superuser token")
	}
	return nil
}
