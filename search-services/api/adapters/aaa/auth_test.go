package aaa

import (
	"log/slog"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		adminUser  string
		adminPass  string
		jwtSecret  string
		wantErr    bool
		wantSecret []byte
	}{
		{
			name:       "success with all env vars",
			adminUser:  "admin",
			adminPass:  "secret",
			jwtSecret:  "my-secret",
			wantErr:    false,
			wantSecret: []byte("my-secret"),
		},
		{
			name:       "success with default JWT_SECRET",
			adminUser:  "admin",
			adminPass:  "secret",
			jwtSecret:  "",
			wantErr:    false,
			wantSecret: []byte("default-insecure-secret"),
		},
		{
			name:      "missing ADMIN_USER",
			adminUser: "",
			adminPass: "secret",
			jwtSecret: "x",
			wantErr:   true,
		},
		{
			name:      "missing ADMIN_PASSWORD",
			adminUser: "admin",
			adminPass: "",
			jwtSecret: "x",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.adminUser != "" {
				t.Setenv("ADMIN_USER", tt.adminUser)
			} else {
				t.Setenv("ADMIN_USER", "")
			}
			if tt.adminPass != "" {
				t.Setenv("ADMIN_PASSWORD", tt.adminPass)
			} else {
				t.Setenv("ADMIN_PASSWORD", "")
			}
			if tt.jwtSecret != "" {
				t.Setenv("JWT_SECRET", tt.jwtSecret)
			} else {
				t.Setenv("JWT_SECRET", "")
			}

			logger := slog.Default()
			aaa, err := New(15*time.Minute, logger)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, aaa)
			assert.Equal(t, tt.wantSecret, aaa.secret)
			assert.Len(t, aaa.users, 1)
			assert.Equal(t, tt.adminPass, aaa.users[tt.adminUser])
		})
	}
}

func TestLogin(t *testing.T) {
	t.Setenv("ADMIN_USER", "alice")
	t.Setenv("ADMIN_PASSWORD", "alicepass")
	t.Setenv("JWT_SECRET", "testsecret")

	aaa, err := New(10*time.Minute, slog.Default())
	require.NoError(t, err)

	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{"valid", "alice", "alicepass", false},
		{"invalid password", "alice", "wrong", true},
		{"unknown user", "bob", "any", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := aaa.Login(tt.username, tt.password)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, token)

			parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				return aaa.secret, nil
			})
			require.NoError(t, err)
			claims, ok := parsed.Claims.(jwt.MapClaims)
			assert.True(t, ok)
			assert.Equal(t, "superuser", claims["sub"])

			exp, ok := claims["exp"].(float64)
			assert.True(t, ok)
			assert.Greater(t, time.Unix(int64(exp), 0), time.Now().Add(9*time.Minute))
			assert.Less(t, time.Unix(int64(exp), 0), time.Now().Add(11*time.Minute))
		})
	}
}

func TestVerify(t *testing.T) {
	t.Setenv("ADMIN_USER", "admin")
	t.Setenv("ADMIN_PASSWORD", "pass")
	t.Setenv("JWT_SECRET", "verysecret")

	aaa, err := New(5*time.Minute, slog.Default())
	require.NoError(t, err)

	validToken, err := aaa.Login("admin", "pass")
	require.NoError(t, err)

	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "superuser",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})
	expiredTokenStr, err := expiredToken.SignedString(aaa.secret)
	require.NoError(t, err)

	wrongMethodToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "superuser",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	wrongMethodStr, _ := wrongMethodToken.SignedString([]byte("rsa-key"))

	badSubjectToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "notsuperuser",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	badSubjectStr, err := badSubjectToken.SignedString(aaa.secret)
	require.NoError(t, err)

	malformedToken := "not-a-valid-token"

	tests := []struct {
		name      string
		token     string
		wantError bool
	}{
		{"valid token", validToken, false},
		{"expired token", expiredTokenStr, true},
		{"wrong signing method", wrongMethodStr, true},
		{"invalid subject", badSubjectStr, true},
		{"malformed token", malformedToken, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := aaa.Verify(tt.token)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
