package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustLoad_Defaults(t *testing.T) {
	os.Clearenv()

	cfg := MustLoad("non_existent.yaml")

	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, "words:8080", cfg.WordsAddress)
	assert.Equal(t, ":8080", cfg.APIServer.Address)
	assert.Equal(t, 5*time.Second, cfg.APIServer.Timeout)
	assert.Equal(t, 100, cfg.SearchRate)
}

func TestMustLoad_EnvOverrides(t *testing.T) {
	os.Clearenv()

	err := os.Setenv("LOG_LEVEL", "DEBUG")
	require.NoError(t, err)
	err = os.Setenv("WORDS_ADDRESS", "localhost:9090")
	require.NoError(t, err)
	err = os.Setenv("API_TIMEOUT", "10s")
	require.NoError(t, err)
	defer os.Clearenv()

	cfg := MustLoad("")

	assert.Equal(t, "DEBUG", cfg.LogLevel)
	assert.Equal(t, "localhost:9090", cfg.WordsAddress)
	assert.Equal(t, 10*time.Second, cfg.APIServer.Timeout)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "Valid config",
			cfg: Config{
				LogLevel:     "INFO",
				WordsAddress: "words:8080",
				APIServer: APIServerConfig{
					Address: ":8080",
					Timeout: time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid LogLevel",
			cfg: Config{
				LogLevel:     "TRACE",
				WordsAddress: "words:8080",
				APIServer:    APIServerConfig{Address: ":80", Timeout: time.Second},
			},
			wantErr: true,
		},
		{
			name: "Empty WordsAddress",
			cfg: Config{
				LogLevel:     "INFO",
				WordsAddress: "",
				APIServer:    APIServerConfig{Address: ":80", Timeout: time.Second},
			},
			wantErr: true,
		},
		{
			name: "Negative Timeout",
			cfg: Config{
				LogLevel:     "INFO",
				WordsAddress: "words:8080",
				APIServer: APIServerConfig{
					Address: ":80",
					Timeout: -1 * time.Second,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
