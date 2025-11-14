package agent

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"win-agent/internal/config"
	natsclient "win-agent/internal/nats"
	"win-agent/internal/scheduler"
	"win-agent/internal/tasks"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Agent represents the main agent
type Agent struct {
	config    *config.Config
	logger    *zap.Logger
	nats      *natsclient.Client
	scheduler *scheduler.Scheduler
	handlers  *natsclient.CommandHandlers
	version   string
}

// New creates a new agent instance
func New(configPath string, version string) (*Agent, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	logger, err := initLogger(cfg.Logging)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Starting win-agent",
		zap.String("version", version),
		zap.String("device_id", cfg.DeviceID))

	// Create task executor
	executor := tasks.NewExecutor(logger)

	// Connect to NATS
	logger.Info("Connecting to NATS...")
	natsClient, err := natsclient.NewClient(&cfg.NATS, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create command handlers
	handlers := natsclient.NewCommandHandlers(logger, cfg, executor)

	// Subscribe to commands
	logger.Info("Subscribing to commands...")
	if err := handlers.SubscribeAll(natsClient); err != nil {
		natsClient.Close()
		return nil, fmt.Errorf("failed to subscribe to commands: %w", err)
	}

	// Create and start scheduler
	logger.Info("Starting scheduler...")
	sched, err := scheduler.New(logger, natsClient, executor, cfg, version)
	if err != nil {
		natsClient.Close()
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	return &Agent{
		config:    cfg,
		logger:    logger,
		nats:      natsClient,
		scheduler: sched,
		handlers:  handlers,
		version:   version,
	}, nil
}

// Run starts the agent and blocks until shutdown
func (a *Agent) Run() error {
	// Start the scheduler
	a.scheduler.Start()

	a.logger.Info("Agent running",
		zap.String("device_id", a.config.DeviceID),
		zap.String("version", a.version))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	a.logger.Info("Received shutdown signal")

	return a.Shutdown()
}

// Shutdown gracefully shuts down the agent
func (a *Agent) Shutdown() error {
	a.logger.Info("Shutting down agent gracefully")

	// Stop accepting new scheduled tasks
	if err := a.scheduler.Shutdown(); err != nil {
		a.logger.Error("Error shutting down scheduler", zap.Error(err))
	}

	// Drain NATS connection (wait for in-flight messages)
	if err := a.nats.Drain(a.config.NATS.DrainTimeout); err != nil {
		a.logger.Error("Error draining NATS", zap.Error(err))
	}

	// Sync logger
	a.logger.Sync()

	a.logger.Info("Agent shutdown complete")
	return nil
}

// initLogger creates and configures the logger
func initLogger(cfg config.LoggingConfig) (*zap.Logger, error) {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create core for file logging
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	// Open log file
	logFile, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create console encoder for stdout (during development)
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create multi-writer core (file + console)
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), level),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}
