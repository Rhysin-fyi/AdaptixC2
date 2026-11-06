package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type recordingTeamserver struct {
	all    []string
	client []string
}

func (r *recordingTeamserver) TsServiceSendDataAll(service string, data string) {
	r.all = append(r.all, data)
}

func (r *recordingTeamserver) TsServiceSendDataClient(operator string, service string, data string) {
	r.client = append(r.client, data)
}

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

func TestSendDestinationsIncludesConnectionStateWithoutToken(t *testing.T) {
	teamserver := &recordingTeamserver{}
	config := validTestConfig()
	plugin := &Plugin{
		ts:     teamserver,
		config: config,
		states: map[string]connectionState{
			"lab-linux": {
				State:       "Online",
				LastSeen:    time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
				LastMessage: "ready",
			},
		},
	}

	plugin.sendDestinations("operator1")
	if len(teamserver.client) != 1 {
		t.Fatalf("client messages = %d, want 1", len(teamserver.client))
	}
	message := teamserver.client[0]
	if strings.Contains(message, config.OperatorBotToken) {
		t.Fatal("destination response exposed the operator bot token")
	}
	for _, expected := range []string{`"state":"Online"`, `"last_message":"ready"`, `"bot_id":123456789`} {
		if !strings.Contains(message, expected) {
			t.Fatalf("destination response %q does not contain %q", message, expected)
		}
	}
}

func TestPublishConnectionRecordsLastSeenAndBroadcasts(t *testing.T) {
	teamserver := &recordingTeamserver{}
	destination := validTestConfig().Destinations[0]
	plugin := &Plugin{ts: teamserver, states: make(map[string]connectionState)}

	plugin.publishConnection(destination, "Online", "ready", true)
	state := plugin.states[destination.Name]
	if state.State != "Online" || state.LastMessage != "ready" || state.LastSeen.IsZero() {
		t.Fatalf("connection state = %#v", state)
	}
	if len(teamserver.all) != 1 || !strings.Contains(teamserver.all[0], `"type":"connection"`) {
		t.Fatalf("broadcast messages = %#v", teamserver.all)
	}
}
