// Copyright 2015 Matthew Holt and The Caddy Authors
// Copyright 2025 K2 <skrik2@outlook.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");

package internal

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogBufferCore is an in-memory zap logger that implements zapcore.Core.
//
// zapcore.Core https://github.com/uber-go/zap/blob/v1.27.1/zapcore/core.go#L25
type LogBufferCore struct {
	mu      sync.Mutex
	entries []zapcore.Entry
	fields  [][]zapcore.Field
	level   zapcore.LevelEnabler
}

// Enabled returns true if the given log level is enabled.
// it implements zapcore.LevelEnabler
//
// zapcore.LevelEnabler https://github.com/uber-go/zap/blob/v1.27.1/zapcore/level.go#L227
func (c *LogBufferCore) Enabled(lvl zapcore.Level) bool {
	return c.level.Enabled(lvl)
}

// With returns a Core with additional structured fields.
// This implementation ignores the fields and returns itself.
func (c *LogBufferCore) With(fields []zapcore.Field) zapcore.Core {
	return c
}

// Check determines if the log entry should be logged.
// If enabled, adds this core to the CheckedEntry.
func (c *LogBufferCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// Write appends the entry and fields to the internal buffer.
func (c *LogBufferCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entry)
	c.fields = append(c.fields, fields)
	return nil
}

// Sync is a no-op for the in-memory buffer.
func (c *LogBufferCore) Sync() error { return nil }

// FlushTo writes all buffered log entries to the given zap.Logger
// and clears the buffer.
func (c *LogBufferCore) FlushTo(logger *zap.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for idx, entry := range c.entries {
		logger.WithOptions().Check(entry.Level, entry.Message).Write(c.fields[idx]...)
	}
	c.entries = nil
	c.fields = nil
}

// LogBufferCoreInterface is a helper interface that combines zapcore.Core
// with the FlushTo method. It allows code to treat a core as flushable
// without knowing the concrete type.
type LogBufferCoreInterface interface {
	zapcore.Core
	// FlushTo writes all buffered log entries to the given zap.Logger.
	FlushTo(*zap.Logger)
}

// NewLogBufferCore creates a new LogBufferCore with the specified log level.
func NewLogBufferCore(level zapcore.LevelEnabler) *LogBufferCore {
	return &LogBufferCore{
		level: level,
	}
}

var (
	_ zapcore.Core           = (*LogBufferCore)(nil)
	_ LogBufferCoreInterface = (*LogBufferCore)(nil)
)
