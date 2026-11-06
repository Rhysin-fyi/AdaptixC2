package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	adaptix "github.com/Adaptix-Framework/axc2"
)

const (
	serviceName          = "TeleC2"
	defaultConfigName    = "telec2-config.json"
	telegramDownloadCeil = 20
)

var safeFilename = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type Teamserver interface {
	TsServiceSendDataAll(service string, data string)
	TsServiceSendDataClient(operator string, service string, data string)
}

type Destination struct {
	Name              string `json:"name"`
	ChatID            int64  `json:"chat_id"`
	TeleC2BotID       int64  `json:"telec2_bot_id"`
	TeleC2BotUsername string `json:"telec2_bot_username"`
	Platform          string `json:"platform"`
}

type Config struct {
	OperatorBotToken   string        `json:"operator_bot_token"`
	MaxFileSizeMB      int           `json:"max_file_size_mb"`
	PollTimeoutSeconds int           `json:"poll_timeout_seconds"`
	Destinations       []Destination `json:"destinations"`
}

type Plugin struct {
	ts         Teamserver
	moduleDir  string
	configFile string
	client     *http.Client

	mu        sync.RWMutex
	config    Config
	configErr error
}

type commandArgs struct {
	Destination string `json:"destination"`
	Filename    string `json:"filename"`
	Content     string `json:"content_base64"`
}

type telegramEnvelope struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64  `json:"message_id"`
		Text      string `json:"text"`
		Caption   string `json:"caption"`
		From      *struct {
			ID       int64  `json:"id"`
			IsBot    bool   `json:"is_bot"`
			Username string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Document *struct {
			FileName string `json:"file_name"`
		} `json:"document"`
	} `json:"message"`
}

func InitPlugin(rawTeamserver any, moduleDir string, serviceConfig string) adaptix.PluginService {
	ts, ok := rawTeamserver.(Teamserver)
	if !ok {
		fmt.Fprintln(os.Stderr, "TeleC2 service: unsupported teamserver interface")
		return nil
	}

	if serviceConfig == "" {
		serviceConfig = defaultConfigName
	}
	p := &Plugin{
		ts:         ts,
		moduleDir:  moduleDir,
		configFile: serviceConfig,
		client:     &http.Client{Timeout: 60 * time.Second},
	}
	p.reloadConfig()
	go p.pollUpdates()
	return p
}

func (p *Plugin) Call(operator string, function string, argsJSON string) {
	switch function {
	case "list":
		p.sendDestinations(operator)
	case "reload":
		p.reloadConfig()
		p.sendDestinations(operator)
	case "status":
		p.requestStatus(operator, argsJSON)
	case "upload":
		p.upload(operator, argsJSON)
	default:
		p.send(operator, map[string]any{"type": "result", "ok": false, "action": function, "message": "Unsupported TeleC2 action"})
	}
}

func (p *Plugin) configPath() (string, error) {
	if filepath.IsAbs(p.configFile) {
		return "", errors.New("service_config must be a filename inside the TeleC2 extender directory")
	}
	clean := filepath.Clean(p.configFile)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid TeleC2 service_config path")
	}
	return filepath.Join(p.moduleDir, clean), nil
}

func (p *Plugin) reloadConfig() {
	config, err := p.loadConfig()
	p.mu.Lock()
	p.config = config
	p.configErr = err
	p.mu.Unlock()
}

func (p *Plugin) loadConfig() (Config, error) {
	path, err := p.configPath()
	if err != nil {
		return Config{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("configuration missing: copy telec2-config.example.json to %s and chmod 600", filepath.Base(path))
		}
		return Config{}, fmt.Errorf("cannot inspect configuration: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Config{}, errors.New("telec2-config.json must not be readable or writable by group or other users; run chmod 600")
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("cannot open configuration: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	if err := validateConfig(&config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func validateConfig(config *Config) error {
	config.OperatorBotToken = strings.TrimSpace(config.OperatorBotToken)
	if config.OperatorBotToken == "" || strings.HasPrefix(config.OperatorBotToken, "REPLACE_") {
		return errors.New("operator_bot_token is not configured")
	}
	if config.MaxFileSizeMB == 0 {
		config.MaxFileSizeMB = telegramDownloadCeil
	}
	if config.MaxFileSizeMB < 1 || config.MaxFileSizeMB > telegramDownloadCeil {
		return fmt.Errorf("max_file_size_mb must be between 1 and %d", telegramDownloadCeil)
	}
	if config.PollTimeoutSeconds == 0 {
		config.PollTimeoutSeconds = 20
	}
	if config.PollTimeoutSeconds < 5 || config.PollTimeoutSeconds > 50 {
		return errors.New("poll_timeout_seconds must be between 5 and 50")
	}
	if len(config.Destinations) == 0 {
		return errors.New("at least one TeleC2 destination is required")
	}

	names := make(map[string]struct{}, len(config.Destinations))
	for i := range config.Destinations {
		d := &config.Destinations[i]
		d.Name = strings.TrimSpace(d.Name)
		d.TeleC2BotUsername = strings.TrimPrefix(strings.TrimSpace(d.TeleC2BotUsername), "@")
		d.Platform = strings.ToLower(strings.TrimSpace(d.Platform))
		if d.Name == "" || d.ChatID == 0 || d.TeleC2BotID <= 0 || d.TeleC2BotUsername == "" {
			return fmt.Errorf("destination %d is missing a required field", i+1)
		}
		if d.Platform != "windows" && d.Platform != "linux" && d.Platform != "macos" {
			return fmt.Errorf("destination %q has unsupported platform %q", d.Name, d.Platform)
		}
		if _, exists := names[d.Name]; exists {
			return fmt.Errorf("duplicate destination name %q", d.Name)
		}
		names[d.Name] = struct{}{}
	}
	return nil
}

func (p *Plugin) snapshot() (Config, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config, p.configErr
}

func (p *Plugin) destination(name string) (Config, Destination, error) {
	config, err := p.snapshot()
	if err != nil {
		return Config{}, Destination{}, err
	}
	for _, destination := range config.Destinations {
		if destination.Name == name {
			return config, destination, nil
		}
	}
	return Config{}, Destination{}, fmt.Errorf("unknown TeleC2 destination %q", name)
}

func (p *Plugin) sendDestinations(operator string) {
	config, err := p.snapshot()
	if err != nil {
		p.send(operator, map[string]any{"type": "config", "ok": false, "message": err.Error(), "destinations": []any{}})
		return
	}
	items := make([]map[string]any, 0, len(config.Destinations))
	for _, destination := range config.Destinations {
		items = append(items, map[string]any{
			"name": destination.Name, "platform": destination.Platform,
			"bot_username": destination.TeleC2BotUsername,
		})
	}
	p.send(operator, map[string]any{
		"type": "config", "ok": true, "destinations": items,
		"max_file_size_bytes": int64(config.MaxFileSizeMB) * 1024 * 1024,
	})
}

func decodeArgs(argsJSON string) (commandArgs, error) {
	var args commandArgs
	decoder := json.NewDecoder(strings.NewReader(argsJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return commandArgs{}, errors.New("invalid TeleC2 command arguments")
	}
	args.Destination = strings.TrimSpace(args.Destination)
	if args.Destination == "" {
		return commandArgs{}, errors.New("destination is required")
	}
	return args, nil
}

func (p *Plugin) requestStatus(operator string, argsJSON string) {
	args, err := decodeArgs(argsJSON)
	if err != nil {
		p.result(operator, "status", false, err.Error(), nil)
		return
	}
	config, destination, err := p.destination(args.Destination)
	if err != nil {
		p.result(operator, "status", false, err.Error(), nil)
		return
	}
	message := "/status@" + destination.TeleC2BotUsername
	if err := p.sendTelegramMessage(config, destination.ChatID, message); err != nil {
		p.result(operator, "status", false, err.Error(), map[string]any{"destination": destination.Name})
		return
	}
	p.result(operator, "status", true, "Status request sent", map[string]any{"destination": destination.Name})
}

func (p *Plugin) upload(operator string, argsJSON string) {
	args, err := decodeArgs(argsJSON)
	if err != nil {
		p.result(operator, "upload", false, err.Error(), nil)
		return
	}
	config, destination, err := p.destination(args.Destination)
	if err != nil {
		p.result(operator, "upload", false, err.Error(), nil)
		return
	}
	filename := sanitizeFilename(args.Filename)
	if filename == "" {
		p.result(operator, "upload", false, "filename is invalid", nil)
		return
	}
	maxBytes := int64(config.MaxFileSizeMB) * 1024 * 1024
	if int64(base64.StdEncoding.DecodedLen(len(args.Content))) > maxBytes+2 {
		p.result(operator, "upload", false, "file exceeds the configured TeleC2 limit", nil)
		return
	}
	content, err := base64.StdEncoding.Strict().DecodeString(args.Content)
	if err != nil {
		p.result(operator, "upload", false, "file content is not valid Base64", nil)
		return
	}
	if len(content) == 0 {
		p.result(operator, "upload", false, "empty files are not uploaded", nil)
		return
	}
	if int64(len(content)) > maxBytes {
		p.result(operator, "upload", false, "file exceeds the configured TeleC2 limit", nil)
		return
	}

	digest := sha256.Sum256(content)
	if err := p.sendTelegramDocument(config, destination.ChatID, filename, content); err != nil {
		p.result(operator, "upload", false, err.Error(), map[string]any{"destination": destination.Name})
		return
	}
	p.result(operator, "upload", true, "Telegram accepted the document; waiting for TeleC2 confirmation", map[string]any{
		"destination": destination.Name,
		"filename":    filename,
		"size":        len(content),
		"sha256":      hex.EncodeToString(digest[:]),
	})
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Trim(safeFilename.ReplaceAllString(name, "_"), "._-")
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

func (p *Plugin) telegramRequest(config Config, method string, body io.Reader, contentType string) (json.RawMessage, error) {
	endpoint := "https://api.telegram.org/bot" + config.OperatorBotToken + "/" + method
	request, err := http.NewRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return nil, errors.New("could not create Telegram request")
	}
	request.Header.Set("Content-Type", contentType)
	response, err := p.client.Do(request)
	if err != nil {
		return nil, errors.New("Telegram request failed")
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 2<<20)
	var envelope telegramEnvelope
	if err := json.NewDecoder(limited).Decode(&envelope); err != nil {
		return nil, errors.New("Telegram returned an invalid response")
	}
	if !envelope.OK {
		description := strings.TrimSpace(envelope.Description)
		if description == "" {
			description = "request rejected"
		}
		return nil, errors.New("Telegram: " + description)
	}
	return envelope.Result, nil
}

func (p *Plugin) sendTelegramMessage(config Config, chatID int64, message string) error {
	values := url.Values{}
	values.Set("chat_id", strconv.FormatInt(chatID, 10))
	values.Set("text", message)
	_, err := p.telegramRequest(config, "sendMessage", strings.NewReader(values.Encode()), "application/x-www-form-urlencoded")
	return err
}

func (p *Plugin) sendTelegramDocument(config Config, chatID int64, filename string, content []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return errors.New("could not create Telegram upload")
	}
	part, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return errors.New("could not create Telegram upload")
	}
	if _, err := part.Write(content); err != nil {
		return errors.New("could not create Telegram upload")
	}
	if err := writer.Close(); err != nil {
		return errors.New("could not finalize Telegram upload")
	}
	_, err = p.telegramRequest(config, "sendDocument", &body, writer.FormDataContentType())
	return err
}

func (p *Plugin) pollUpdates() {
	var offset int64
	var activeToken string
	for {
		config, err := p.snapshot()
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		if config.OperatorBotToken != activeToken {
			activeToken = config.OperatorBotToken
			offset = 0
		}
		updates, err := p.getUpdates(config, offset)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			p.forwardUpdate(config, update)
		}
	}
}

func (p *Plugin) getUpdates(config Config, offset int64) ([]telegramUpdate, error) {
	values := url.Values{}
	values.Set("offset", strconv.FormatInt(offset, 10))
	values.Set("timeout", strconv.Itoa(config.PollTimeoutSeconds))
	values.Set("allowed_updates", `["message"]`)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.PollTimeoutSeconds+5)*time.Second)
	defer cancel()
	endpoint := "https://api.telegram.org/bot" + config.OperatorBotToken + "/getUpdates"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var envelope telegramEnvelope
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&envelope); err != nil {
		return nil, err
	}
	if !envelope.OK {
		return nil, errors.New("Telegram rejected getUpdates")
	}
	var updates []telegramUpdate
	if err := json.Unmarshal(envelope.Result, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (p *Plugin) forwardUpdate(config Config, update telegramUpdate) {
	if update.Message == nil || update.Message.From == nil || !update.Message.From.IsBot {
		return
	}
	var destination *Destination
	for i := range config.Destinations {
		candidate := &config.Destinations[i]
		if candidate.ChatID == update.Message.Chat.ID && candidate.TeleC2BotID == update.Message.From.ID {
			destination = candidate
			break
		}
	}
	if destination == nil {
		return
	}
	message := strings.TrimSpace(update.Message.Text)
	if message == "" {
		message = strings.TrimSpace(update.Message.Caption)
	}
	if message == "" && update.Message.Document != nil {
		message = "Document: " + update.Message.Document.FileName
	}
	if message == "" {
		message = "TeleC2 sent an update"
	}
	p.sendAll(map[string]any{
		"type": "telec2_reply", "ok": true, "destination": destination.Name,
		"message": message,
	})
}

func (p *Plugin) result(operator string, action string, ok bool, message string, extra map[string]any) {
	payload := map[string]any{"type": "result", "action": action, "ok": ok, "message": message}
	for key, value := range extra {
		payload[key] = value
	}
	p.send(operator, payload)
}

func (p *Plugin) send(operator string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err == nil {
		p.ts.TsServiceSendDataClient(operator, serviceName, string(data))
	}
}

func (p *Plugin) sendAll(payload map[string]any) {
	data, err := json.Marshal(payload)
	if err == nil {
		p.ts.TsServiceSendDataAll(serviceName, string(data))
	}
}
