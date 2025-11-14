package nats

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"win-agent/internal/config"
	"win-agent/internal/tasks"
	"go.uber.org/zap"
)

// CommandHandlers manages all command subscriptions and handlers
type CommandHandlers struct {
	logger       *zap.Logger
	config       *config.Config
	deviceID     string
	taskExecutor *tasks.Executor
}

// NewCommandHandlers creates a new command handler manager
func NewCommandHandlers(logger *zap.Logger, cfg *config.Config, executor *tasks.Executor) *CommandHandlers {
	return &CommandHandlers{
		logger:       logger,
		config:       cfg,
		deviceID:     cfg.DeviceID,
		taskExecutor: executor,
	}
}

// SubscribeAll subscribes to all command subjects for this device
func (h *CommandHandlers) SubscribeAll(client *Client) error {
	// Subscribe to ping command
	if _, err := client.Subscribe(
		fmt.Sprintf("agents.%s.cmd.ping", h.deviceID),
		h.handlePing,
	); err != nil {
		return err
	}

	// Subscribe to service control command
	if _, err := client.Subscribe(
		fmt.Sprintf("agents.%s.cmd.service", h.deviceID),
		h.handleServiceControl,
	); err != nil {
		return err
	}

	// Subscribe to log fetch command
	if _, err := client.Subscribe(
		fmt.Sprintf("agents.%s.cmd.logs", h.deviceID),
		h.handleLogFetch,
	); err != nil {
		return err
	}

	// Subscribe to custom exec command
	if _, err := client.Subscribe(
		fmt.Sprintf("agents.%s.cmd.exec", h.deviceID),
		h.handleCustomExec,
	); err != nil {
		return err
	}

	return nil
}

// Response structures

type pingResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type serviceControlRequest struct {
	Action      string `json:"action"`
	ServiceName string `json:"service_name"`
}

type serviceControlResponse struct {
	Status      string `json:"status"`
	ServiceName string `json:"service_name,omitempty"`
	Action      string `json:"action,omitempty"`
	Result      string `json:"result,omitempty"`
	Error       string `json:"error,omitempty"`
	Timestamp   string `json:"timestamp"`
}

type logFetchRequest struct {
	LogPath string `json:"log_path"`
	Lines   int    `json:"lines"`
}

type logFetchResponse struct {
	Status     string   `json:"status"`
	LogPath    string   `json:"log_path,omitempty"`
	Lines      []string `json:"lines,omitempty"`
	TotalLines int      `json:"total_lines,omitempty"`
	Error      string   `json:"error,omitempty"`
	Timestamp  string   `json:"timestamp"`
}

type customExecRequest struct {
	Command string `json:"command"`
}

type customExecResponse struct {
	Status    string `json:"status"`
	Command   string `json:"command,omitempty"`
	Output    string `json:"output,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

type errorResponse struct {
	Status    string `json:"status"`
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

// handlePing responds to ping commands
func (h *CommandHandlers) handlePing(msg *nats.Msg) {
	h.logger.Debug("Received ping command")

	response := pingResponse{
		Status:    "pong",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	responseBytes, _ := json.Marshal(response)
	msg.Respond(responseBytes)

	h.logger.Debug("Sent pong response")
}

// handleServiceControl processes service start/stop/restart commands
func (h *CommandHandlers) handleServiceControl(msg *nats.Msg) {
	h.logger.Debug("Received service control command")

	// Parse request
	var req serviceControlRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.logger.Error("Failed to parse service control request", zap.Error(err))
		h.respondError(msg, "Invalid request format")
		return
	}

	h.logger.Info("Processing service control",
		zap.String("action", req.Action),
		zap.String("service", req.ServiceName))

	// Execute service control
	result, err := h.taskExecutor.ControlService(req.ServiceName, req.Action, h.config.Commands.AllowedServices)
	if err != nil {
		h.logger.Error("Service control failed",
			zap.Error(err),
			zap.String("service", req.ServiceName),
			zap.String("action", req.Action))

		response := serviceControlResponse{
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		responseBytes, _ := json.Marshal(response)
		msg.Respond(responseBytes)
		return
	}

	// Success response
	response := serviceControlResponse{
		Status:      "success",
		ServiceName: req.ServiceName,
		Action:      req.Action,
		Result:      result,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	responseBytes, _ := json.Marshal(response)
	msg.Respond(responseBytes)

	h.logger.Info("Service control succeeded",
		zap.String("service", req.ServiceName),
		zap.String("action", req.Action))
}

// handleLogFetch retrieves log file contents
func (h *CommandHandlers) handleLogFetch(msg *nats.Msg) {
	h.logger.Debug("Received log fetch command")

	// Parse request
	var req logFetchRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.logger.Error("Failed to parse log fetch request", zap.Error(err))
		h.respondError(msg, "Invalid request format")
		return
	}

	h.logger.Info("Fetching log file",
		zap.String("path", req.LogPath),
		zap.Int("lines", req.Lines))

	// Fetch log lines
	lines, err := h.taskExecutor.FetchLogLines(req.LogPath, req.Lines, h.config.Commands.AllowedLogPaths)
	if err != nil {
		h.logger.Error("Log fetch failed",
			zap.Error(err),
			zap.String("path", req.LogPath))

		response := logFetchResponse{
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		responseBytes, _ := json.Marshal(response)
		msg.Respond(responseBytes)
		return
	}

	// Success response
	response := logFetchResponse{
		Status:     "success",
		LogPath:    req.LogPath,
		Lines:      lines,
		TotalLines: len(lines),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	responseBytes, _ := json.Marshal(response)
	msg.Respond(responseBytes)

	h.logger.Info("Log fetch succeeded",
		zap.String("path", req.LogPath),
		zap.Int("lines", len(lines)))
}

// handleCustomExec executes whitelisted PowerShell commands
func (h *CommandHandlers) handleCustomExec(msg *nats.Msg) {
	h.logger.Debug("Received custom exec command")

	// Parse request
	var req customExecRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.logger.Error("Failed to parse exec request", zap.Error(err))
		h.respondError(msg, "Invalid request format")
		return
	}

	h.logger.Info("Executing custom command", zap.String("command", req.Command))

	// Execute command
	output, exitCode, err := h.taskExecutor.ExecuteCommand(req.Command, h.config.Commands.AllowedCommands)
	if err != nil {
		h.logger.Error("Command execution failed",
			zap.Error(err),
			zap.String("command", req.Command))

		response := customExecResponse{
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		responseBytes, _ := json.Marshal(response)
		msg.Respond(responseBytes)
		return
	}

	// Success response
	response := customExecResponse{
		Status:    "success",
		Command:   req.Command,
		Output:    output,
		ExitCode:  exitCode,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	responseBytes, _ := json.Marshal(response)
	msg.Respond(responseBytes)

	h.logger.Info("Command execution succeeded",
		zap.String("command", req.Command),
		zap.Int("exit_code", exitCode))
}

// respondError sends a generic error response
func (h *CommandHandlers) respondError(msg *nats.Msg, errorMsg string) {
	response := errorResponse{
		Status:    "error",
		Error:     errorMsg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	responseBytes, _ := json.Marshal(response)
	msg.Respond(responseBytes)
}
