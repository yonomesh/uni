package logging

import (
	"github.com/yonomesh/uni"

	"go.uber.org/zap/zapcore"
)

func init() {
	uni.RegisterModule(MockCore{})
}

// MockCore is a no-op module, purely for testing
type MockCore struct {
	zapcore.Core `json:"-"`
}

// CaddyModule returns the Caddy module information.
func (MockCore) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "logging.cores.mock",
		New: func() uni.Module { return new(MockCore) },
	}
}

// d *caddyfile.Dispenser
func (lec *MockCore) UnmarshalUniConfigfile() error {
	return nil
}

// Interface guards
var (
	_ zapcore.Core = (*MockCore)(nil)
	_ uni.Module   = (*MockCore)(nil)
	// _ caddyfile.Unmarshaler = (*MockCore)(nil)
)
