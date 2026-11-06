package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func validTestConfig() Config {
	return Config{
		OperatorBotToken: "123456:test-token",
		Destinations: []Destination{
			{
				Name:              "lab-linux",
				ChatID:            -1001234567890,
				TeleC2BotID:       123456789,
				TeleC2BotUsername: "example_telec2_bot",
				Platform:          "linux",
			},
		},
	}
}

func TestValidateConfigAppliesSafeDefaults(t *testing.T) {
	config := validTestConfig()
	if err := validateConfig(&config); err != nil {
		t.Fatalf("validateConfig returned an error: %v", err)
	}
	if config.MaxFileSizeMB != 20 {
		t.Fatalf("MaxFileSizeMB = %d, want 20", config.MaxFileSizeMB)
	}
	if config.PollTimeoutSeconds != 20 {
		t.Fatalf("PollTimeoutSeconds = %d, want 20", config.PollTimeoutSeconds)
	}
}

func TestValidateConfigRejectsDuplicateDestinations(t *testing.T) {
	config := validTestConfig()
	config.Destinations = append(config.Destinations, config.Destinations[0])
	if err := validateConfig(&config); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("validateConfig error = %v, want duplicate destination error", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	if got, want := sanitizeFilename("../../bad name!.exe"), "bad_name_.exe"; got != want {
		t.Fatalf("sanitizeFilename() = %q, want %q", got, want)
	}
	if got := sanitizeFilename("../..."); got != "" {
		t.Fatalf("sanitizeFilename() = %q, want empty", got)
	}
}

func TestLoadConfigRequiresPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not enforced on Windows")
	}
	directory := t.TempDir()
	config := validTestConfig()
	config.MaxFileSizeMB = 20
	config.PollTimeoutSeconds = 20
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, defaultConfigName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	plugin := &Plugin{moduleDir: directory, configFile: defaultConfigName}
	if _, err := plugin.loadConfig(); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("loadConfig error = %v, want private-permissions error", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.loadConfig(); err != nil {
		t.Fatalf("loadConfig with mode 0600 returned error: %v", err)
	}
}
