package config

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMustLoad(t *testing.T) {
	t.Run("Load_From_Env_Default", func(t *testing.T) {
		err := os.Unsetenv("WORDS_ADDRESS")
		require.NoError(t, err)

		cfg := MustLoad("config.yaml")

		require.Equal(t, ":8080", cfg.Address, "expected default address")
	})

	t.Run("Load_From_Env_Custom", func(t *testing.T) {
		expected := ":9090"
		err := os.Setenv("WORDS_ADDRESS", expected)
		require.NoError(t, err)

		defer func() {
			err := os.Unsetenv("WORDS_ADDRESS")
			if err != nil {
				t.Logf("failed to unset WORDS_ADDRESS: %v", err)
			}
		}()

		cfg := MustLoad("config.yaml")
		require.Equal(t, expected, cfg.Address, "expected custom address from env")
	})

	t.Run("Load_From_File", func(t *testing.T) {
		content := []byte("words_address: \":7070\"")
		tmpFile := "test_config.yaml"
		err := os.WriteFile(tmpFile, content, 0644)
		require.NoError(t, err)

		defer func() {
			err := os.Remove(tmpFile)
			if err != nil {
				t.Logf("failed to remove temp file: %v", err)
			}
		}()

		cfg := MustLoad(tmpFile)
		require.Equal(t, ":7070", cfg.Address, "expected address from config file")
	})

	t.Run("Load_Default_File_Success", func(t *testing.T) {
		content := []byte("words_address: \":6060\"")
		err := os.WriteFile("config.yaml", content, 0644)
		require.NoError(t, err)

		defer func() {
			err := os.Remove("config.yaml")
			if err != nil {
				t.Logf("failed to remove config.yaml: %v", err)
			}
		}()

		cfg := MustLoad("config.yaml")
		require.Equal(t, ":6060", cfg.Address, "expected address from default config file")
	})
}

func TestMustLoad_NonExistentFile_Exits(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		MustLoad("non_existent_config.yaml")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMustLoad_NonExistentFile_Exits")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok && !exitErr.Success() {
		return
	}
	t.Fatalf("expected process to exit with non-zero code, got %v", err)
}
