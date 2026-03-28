package logging

import (
	"go.uber.org/zap/zapcore"

	"github.com/yonomesh/uni"
)

func init() {
	uni.RegisterModule(ConsoleEncoder{})
}

// ConsoleEncoder encodes log entries that are mostly human-readable.
type ConsoleEncoder struct {
	zapcore.Encoder `json:"-"`
	LogEncoderConfig
}

// UniModule returns the Uni module information.
func (ConsoleEncoder) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.console",
		New: func() uni.Module { return new(ConsoleEncoder) },
	}
}

// Provision sets up the encoder.
func (ce *ConsoleEncoder) Provision(_ uni.Context) error {
	if ce.LevelFormat == "" {
		ce.LevelFormat = "color"
	}
	if ce.TimeFormat == "" {
		ce.TimeFormat = "wall_milli"
	}
	ce.Encoder = zapcore.NewConsoleEncoder(ce.ZapcoreEncoderConfig())
	return nil
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens. Syntax:
//
//	console {
//	    <common encoder config subdirectives...>
//	}
//
// See the godoc on the LogEncoderConfig type for the syntax of
// subdirectives that are common to most/all encoders.
func (ce *ConsoleEncoder) UnmarshalUniConfigfile() error {

	err := ce.LogEncoderConfig.UnmarshalUniConfigfile()
	if err != nil {
		return err
	}
	return nil
}
