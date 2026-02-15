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

// LogBufferCore is a zapcore.Core that buffers log entries in memory.
//
// https://github.com/uber-go/zap/blob/v1.27.1/zapcore/core.go#L25
type LogBufferCore struct {
	mu      sync.Mutex
	entries []zapcore.Entry
	fields  [][]zapcore.Field
	level   zapcore.LevelEnabler
}

func (c *LogBufferCore) Enabled(lvl zapcore.Level) bool {
	return c.level.Enabled(lvl)
}

func (c *LogBufferCore) With(fields []zapcore.Field) zapcore.Core {
	return c
}

func (c *LogBufferCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *LogBufferCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entry)
	c.fields = append(c.fields, fields)
	return nil
}

func (c *LogBufferCore) Sync() error { return nil }

// FlushTo flushes buffered logs to the given zap.Logger.
func (c *LogBufferCore) FlushTo(logger *zap.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for idx, entry := range c.entries {
		logger.WithOptions().Check(entry.Level, entry.Message).Write(c.fields[idx]...)
	}
	c.entries = nil
	c.fields = nil
}

type LogBufferCoreInterface interface {
	zapcore.Core
	FlushTo(*zap.Logger)
}

func NewLogBufferCore(level zapcore.LevelEnabler) *LogBufferCore {
	return &LogBufferCore{
		level: level,
	}
}

var (
	_ zapcore.Core           = (*LogBufferCore)(nil)
	_ LogBufferCoreInterface = (*LogBufferCore)(nil)
)
