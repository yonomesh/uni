package logging

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var bufferpool = buffer.NewPool()

// LogEncoderConfig holds configuration common to most encoders.
type LogEncoderConfig struct {
	MessageKey    *string `json:"message_key,omitempty"`
	LevelKey      *string `json:"level_key,omitempty"`
	TimeKey       *string `json:"time_key,omitempty"`
	NameKey       *string `json:"name_key,omitempty"`
	CallerKey     *string `json:"caller_key,omitempty"`
	StacktraceKey *string `json:"stacktrace_key,omitempty"`
	LineEnding    *string `json:"line_ending,omitempty"`

	// Recognized values are: unix_seconds_float, unix_milli_float, unix_nano, iso8601, rfc3339, rfc3339_nano, wall, wall_milli, wall_nano, common_log.
	// The value may also be custom format per the Go `time` package layout specification, as described [here](https://pkg.go.dev/time#pkg-constants).
	TimeFormat string `json:"time_format,omitempty"`
	TimeLocal  bool   `json:"time_local,omitempty"`

	// Recognized values are: s/second/seconds, ns/nano/nanos, ms/milli/millis, string.
	// Empty and unrecognized value default to seconds.
	DurationFormat string `json:"duration_format,omitempty"`

	// Recognized values are: lower, upper, color.
	// Empty and unrecognized value default to lower.
	LevelFormat string `json:"level_format,omitempty"`
}

// ZapcoreEncoderConfig returns the equivalent zapcore.EncoderConfig.
// If lec is nil, zap.NewProductionEncoderConfig() is returned.
func (lec *LogEncoderConfig) ZapcoreEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	if lec == nil {
		lec = new(LogEncoderConfig)
	}
	if lec.MessageKey != nil {
		cfg.MessageKey = *lec.MessageKey
	}
	if lec.LevelKey != nil {
		cfg.LevelKey = *lec.LevelKey
	}
	if lec.TimeKey != nil {
		cfg.TimeKey = *lec.TimeKey
	}
	if lec.NameKey != nil {
		cfg.NameKey = *lec.NameKey
	}
	if lec.CallerKey != nil {
		cfg.CallerKey = *lec.CallerKey
	}
	if lec.StacktraceKey != nil {
		cfg.StacktraceKey = *lec.StacktraceKey
	}
	if lec.LineEnding != nil {
		cfg.LineEnding = *lec.LineEnding
	}

	// time format
	var timeFormatter zapcore.TimeEncoder
	switch lec.TimeFormat {
	case "", "unix_seconds_float":
		timeFormatter = zapcore.EpochTimeEncoder
	case "unix_milli_float":
		timeFormatter = zapcore.EpochMillisTimeEncoder
	case "unix_nano":
		timeFormatter = zapcore.EpochNanosTimeEncoder
	case "iso8601":
		timeFormatter = zapcore.ISO8601TimeEncoder
	default:
		timeFormat := lec.TimeFormat
		switch lec.TimeFormat {
		case "rfc3339":
			timeFormat = time.RFC3339
		case "rfc3339_nano":
			timeFormat = time.RFC3339Nano
		case "wall":
			timeFormat = "2006/01/02 15:04:05"
		case "wall_milli":
			timeFormat = "2006/01/02 15:04:05.000"
		case "wall_nano":
			timeFormat = "2006/01/02 15:04:05.000000000"
		case "common_log":
			timeFormat = "02/Jan/2006:15:04:05 -0700"
		}
		timeFormatter = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			var time time.Time
			if lec.TimeLocal {
				time = ts.Local()
			} else {
				time = ts.UTC()
			}
			encoder.AppendString(time.Format(timeFormat))
		}
	}
	cfg.EncodeTime = timeFormatter

	// duration format
	var durFormatter zapcore.DurationEncoder
	switch lec.DurationFormat {
	case "s", "second", "seconds":
		durFormatter = zapcore.SecondsDurationEncoder
	case "ns", "nano", "nanos":
		durFormatter = zapcore.NanosDurationEncoder
	case "ms", "milli", "millis":
		durFormatter = zapcore.MillisDurationEncoder
	case "string":
		durFormatter = zapcore.StringDurationEncoder
	default:
		durFormatter = zapcore.SecondsDurationEncoder
	}
	cfg.EncodeDuration = durFormatter

	// level format
	var levelFormatter zapcore.LevelEncoder
	switch lec.LevelFormat {
	case "", "lower":
		levelFormatter = zapcore.LowercaseLevelEncoder
	case "upper":
		levelFormatter = zapcore.CapitalLevelEncoder
	case "color":
		levelFormatter = zapcore.CapitalColorLevelEncoder
	}
	cfg.EncodeLevel = levelFormatter

	return cfg
}

// TODO: get config
func (lec *LogEncoderConfig) UnmarshalUniConfigfile() error {
	return nil
}
