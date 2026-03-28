package logging

import (
	"go.uber.org/zap/zapcore"

	"github.com/yonomesh/uni"
)

func init() {
	uni.RegisterModule(JSONEncoder{})
}

// JSONEncoder encodes entries as JSON.
type JSONEncoder struct {
	zapcore.Encoder `json:"-"`
	LogEncoderConfig
}

// UniModule returns the Uni module information.
func (JSONEncoder) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.json",
		New: func() uni.Module { return new(JSONEncoder) },
	}
}

// Provision sets up the encoder.
func (je *JSONEncoder) Provision(_ uni.Context) error {
	je.Encoder = zapcore.NewJSONEncoder(je.ZapcoreEncoderConfig())
	return nil
}

// TODO
//
// UnmarshalCaddyfile sets up the module from Caddyfile tokens. Syntax:
//
//	json {
//	    <common encoder config subdirectives...>
//	}
//
// See the godoc on the LogEncoderConfig type for the syntax of
// subdirectives that are common to most/all encoders.
func (je *JSONEncoder) UnmarshalCaddyfile() error {
	// TODO
	err := je.LogEncoderConfig.UnmarshalUniConfigfile()
	if err != nil {
		return err
	}
	return nil
}

// Interface Guard
var _ zapcore.Encoder = (*JSONEncoder)(nil)
