// Package logger provides a zap-based application logger.
package logger

import "go.uber.org/zap"

// Log is the global zap logger used across the project.
var Log *zap.Logger

// Init configures the global logger in production mode.
func Init() {
	var err error
	Log, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}
