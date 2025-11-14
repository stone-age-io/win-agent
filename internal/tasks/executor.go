package tasks

import (
	"go.uber.org/zap"
)

// Executor handles all task execution for both scheduled tasks and commands
type Executor struct {
	logger *zap.Logger
}

// NewExecutor creates a new task executor
func NewExecutor(logger *zap.Logger) *Executor {
	return &Executor{
		logger: logger,
	}
}
