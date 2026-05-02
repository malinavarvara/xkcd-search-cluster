package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustLoad_Env(t *testing.T) {
	os.Clearenv()

	err := os.Setenv("SEARCH_ADDRESS", "localhost:9090")
	require.NoError(t, err, "failed to set SEARCH_ADDRESS")
	err = os.Setenv("LOG_LEVEL", "INFO")
	require.NoError(t, err, "failed to set LOG_LEVEL")
	err = os.Setenv("INDEX_TTL", "12h")
	require.NoError(t, err, "failed to set INDEX_TTL")

	defer os.Clearenv()

	cfg := MustLoad("")

	assert.Equal(t, "localhost:9090", cfg.Address, "address mismatch")
	assert.Equal(t, "INFO", cfg.LogLevel, "log level mismatch")
	assert.Equal(t, 12*time.Hour, cfg.IndexTTL, "index TTL mismatch")
}

func TestMustLoad_Defaults(t *testing.T) {
	os.Clearenv()

	cfg := MustLoad("")

	assert.Equal(t, "localhost:80", cfg.Address, "default address mismatch")
	assert.Equal(t, "DEBUG", cfg.LogLevel, "default log level mismatch")
	assert.Equal(t, 24*time.Hour, cfg.IndexTTL, "default index TTL mismatch")
}

func TestMustLoad_FromFile(t *testing.T) {
	content := `log_level: ERROR
address: 127.0.0.1:5000
index_rebuild_interval: 30m`

	tmpFile, err := os.CreateTemp("", "config_test.*.yaml")
	require.NoError(t, err, "failed to create temp file")
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}()

	_, err = tmpFile.Write([]byte(content))
	require.NoError(t, err, "failed to write to temp file")
	err = tmpFile.Close()
	require.NoError(t, err, "failed to close temp file")

	cfg := MustLoad(tmpFile.Name())

	assert.Equal(t, "ERROR", cfg.LogLevel, "log level from file mismatch")
	assert.Equal(t, "127.0.0.1:5000", cfg.Address, "address from file mismatch")
	assert.Equal(t, 30*time.Minute, cfg.IndexRebuildInterval, "index rebuild interval from file mismatch")
}
