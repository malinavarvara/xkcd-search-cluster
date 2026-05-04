package config

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMustLoad(t *testing.T) {
	err := os.Unsetenv("UPDATE_ADDRESS")
	require.NoError(t, err, "failed to unset UPDATE_ADDRESS")
	err = os.Unsetenv("LOG_LEVEL")
	require.NoError(t, err, "failed to unset LOG_LEVEL")

	t.Run("LoadFromEnv_Success", func(t *testing.T) {
		const expectedAddr = "localhost:1234"
		err := os.Setenv("UPDATE_ADDRESS", expectedAddr)
		require.NoError(t, err, "failed to set UPDATE_ADDRESS")

		defer func() {
			if err := os.Unsetenv("UPDATE_ADDRESS"); err != nil {
				t.Logf("failed to unset UPDATE_ADDRESS: %v", err)
			}
		}()

		cfg := MustLoad("")
		require.Equal(t, expectedAddr, cfg.Address, "config address mismatch")
	})

	t.Run("LoadFromFile_Success", func(t *testing.T) {
		content := `update_address: "localhost:9090"
db_address: "localhost:9091"
xkcd:
  url: "https://test.xkcd.com"
  concurrency: 5`
		tmpFile := "test_config.yaml"
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err, "failed to write temp config file")

		defer func() {
			if err := os.Remove(tmpFile); err != nil {
				t.Logf("failed to remove temp file: %v", err)
			}
		}()

		cfg := MustLoad(tmpFile)
		require.Equal(t, "localhost:9090", cfg.Address, "config address from file mismatch")
	})
}

func TestMustLoad_NonExistentFile_Exits(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		MustLoad("non_existent.yaml")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMustLoad_NonExistentFile_Exits")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if ok && !exitErr.Success() {
		return
	}
	t.Fatalf("expected process to exit with non-zero code, got %v", err)
}

func TestMustLoad_InvalidYAML_Exits(t *testing.T) {
	tmpFile := "invalid.yaml"
	content := "update_address: [unclosed list"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err, "failed to write invalid YAML file")

	defer func() {
		if err := os.Remove(tmpFile); err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}()

	if os.Getenv("TEST_SUBPROCESS") == "1" {
		MustLoad(tmpFile)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMustLoad_InvalidYAML_Exits")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	err = cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if ok && !exitErr.Success() {
		return
	}
	t.Fatalf("expected exit with error for invalid YAML, got %v", err)
}
