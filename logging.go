package uni

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/yonomesh/uni/internal"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
)

func init() {
	RegisterModule(StdoutWriter{})
	RegisterModule(StderrWriter{})
	RegisterModule(DiscardWriter{})
}

// Log returns the current default logger
func Log() *zap.Logger {
	defaultLoggerMu.RLock()
	defer defaultLoggerMu.RUnlock()
	return defaultLogger.logger
}

// Logging facilitates logging within Uni. The default log is
// called "default" and you can customize it. You can also define
// additional logs.
//
// By default, all logs at INFO level and higher are written to
// standard error ("stderr" writer) in a human-readable format
// ("console" encoder if stdout is an interactive terminal, "json"
// encoder otherwise).
//
// All defined logs accept all log entries by default, but you
// can filter by level and module/logger names. A logger's name
// is the same as the module's name, but a module may append to
// logger names for more specificity. For example, you can
// filter logs emitted only by HTTP handlers using the name
// "http.handlers", because all HTTP handler module names have
// that prefix.
//
// Uni logs (except the sink) are zero-allocation, so they are
// very high-performing in terms of memory and CPU time. Enabling
// sampling can further increase throughput on extremely high-load
// servers.
type Logging struct {
	// define how to record unstructured logs
	//
	// Sink is the destination for all unstructured logs emitted
	// from Go's standard library logger. These logs are common
	// in dependencies that are not designed specifically for use
	// in Uni. Because it is global and unstructured, the sink
	// lacks most advanced features and customizations.
	Sink *SinkLog `json:"sink,omitempty"`

	// Customizing the log
	//
	// Logs are your logs, keyed by an arbitrary name of your
	// choosing. The default log can be customized by defining
	// a log called "default". You can further define other logs
	// and filter what kinds of entries they accept.
	Logs map[string]*CustomLog `json:"logs,omitempty"`

	// This ensures that open log streams can be properly closed when the
	// log configuration is no longer needed, thereby preventing resource leaks.
	//
	// WriterIDs a list of all IDs for open writers; all writers
	// that are opened to provision this logging config
	// must have their keys added to this list so they
	// can be closed when cleaning up
	WriterIDs []string
}

// SinkLog configures the default Go standard library
// global logger in the log package. This is necessary because
// module dependencies which are not built specifically for
// Kaze will use the standard logger. This is also known as
// the "sink" logger.
type SinkLog struct {
	BaseLog
}

// CustomLog represents a custom logger configuration.
//
// By default, a log will emit all log entries. Some entries
// will be skipped if sampling is enabled. Further, the Include
// and Exclude parameters define which loggers (by name) are
// allowed or rejected from emitting in this log. If both Include
// and Exclude are populated, their values must be mutually
// exclusive, and longer namespaces have priority. If neither
// are populated, all logs are emitted.
type CustomLog struct {
	BaseLog

	// Include defines the names of loggers to emit in this
	// log. For example, to include only logs emitted by the
	// admin API, you would include "admin.api".
	Include []string `json:"include,omitempty"`

	// Exclude defines the names of loggers that should be
	// skipped by this log. For example, to exclude only
	// HTTP access logs, you would exclude "http.log.access".
	Exclude []string `json:"exclude,omitempty"`
}

// BaseLog contains the common logging parameters for logging.
type BaseLog struct {
	// The module that writes out log entries for the sink.
	WriterRaw json.RawMessage `json:"writer,omitempty" caddy:"namespace=caddy.logging.writers inline_key=output"`

	// The encoder is how the log entries are formatted or encoded.
	EncoderRaw json.RawMessage `json:"encoder,omitempty" caddy:"namespace=caddy.logging.encoders inline_key=format"`

	// Tees entries through a zap.Core module which can extract
	// log entry metadata and fields for further processing.
	CoreRaw json.RawMessage `json:"core,omitempty" caddy:"namespace=caddy.logging.cores inline_key=module"`

	// Level is the minimum level to emit, and is inclusive.
	// Possible levels: DEBUG, INFO, WARN, ERROR, PANIC, and FATAL
	Level string `json:"level,omitempty"`

	// Sampling configures log entry sampling. If enabled,
	// only some log entries will be emitted. This is useful
	// for improving performance on extremely high-pressure
	// servers.
	Sampling *LogSampling `json:"sampling,omitempty"`

	// If true, the log entry will include the caller's
	// file name and line number. Default off.
	WithCaller bool `json:"with_caller,omitempty"`

	// If non-zero, and `with_caller` is true, this many
	// stack frames will be skipped when determining the
	// caller. Default 0.
	WithCallerSkip int `json:"with_caller_skip,omitempty"`

	// If not empty, the log entry will include a stack trace
	// for all logs at the given level or higher. See `level`
	// for possible values.
	//
	// Default off.
	WithStacktrace string `json:"with_stacktrace,omitempty"`

	// Factory that opens the log writer.
	writerFactory WriterFactory
	// Runtime writer used for log output.
	writer io.WriteCloser

	encoder      zapcore.Encoder
	levelEnabler zapcore.LevelEnabler
	core         zapcore.Core
}

func (cl *BaseLog) buildCore() {
	// logs which only discard their output don't need
	// to perform encoding or any other processing steps
	// at all, so just shortcut to a nop core instead
	if _, ok := cl.writerFactory.(*DiscardWriter); ok {
		cl.core = zapcore.NewNopCore()
		return
	}
	c := zapcore.NewCore(cl.encoder, zapcore.AddSync(cl.writer), cl.levelEnabler)
	if cl.Sampling != nil {
		if cl.Sampling.Interval == 0 {
			cl.Sampling.Interval = 1 * time.Second
		}
		if cl.Sampling.First == 0 {
			cl.Sampling.First = 100
		}
		if cl.Sampling.Thereafter == 0 {
			cl.Sampling.Thereafter = 100
		}
		c = zapcore.NewSamplerWithOptions(c, cl.Sampling.Interval, cl.Sampling.First, cl.Sampling.Thereafter)
	}
	cl.core = c
}

// WriterFactory creates log writers from configuration.
// Implementations describe the writer destination and
// can open a runtime writer instance for log output.
type WriterFactory interface {
	// human-readable descriptions
	fmt.Stringer

	// WriterID returns a string that uniquely ID this
	// writer configuration. It is not shown to humans.
	WriterID() string

	// OpenWriter opens a log for writing. The writer
	// should be safe for concurrent use but need not
	// be synchronous.
	OpenWriter() (io.WriteCloser, error)
}

// IsWriterStandardStream returns true if the input is a
// writer-opener to a standard stream (stdout, stderr).
func IsWriterStandardStream(wo WriterFactory) bool {
	switch wo.(type) {
	case StdoutWriter, StderrWriter,
		*StdoutWriter, *StderrWriter:
		return true
	}
	return false
}

// LogSampling configures log entry sampling.
type LogSampling struct {
	// The window over which to conduct sampling.
	Interval time.Duration `json:"interval,omitempty"`

	// Record the number of logs that are the first to
	// arrive at the same level and message, at
	// each sampling interval.
	First int `json:"first,omitempty"`

	// If more entries with the same level and message
	// are seen during the same interval, keep one in
	// this many entries until the end of the interval.
	Thereafter int `json:"thereafter,omitempty"`
}

// Log represents the log data format.
type LogEntry struct {
	Time     string   `json:"ts"`       // Timestamp of the log entry
	Level    string   `json:"level"`    // Log level (e.g., Trace, Debug, Info, Warning, Error, Fataland Panic)
	Category string   `json:"category"` // Category or type of the log (e.g., user-action)
	Tags     []string `json:"tags"`     // Tags related to the log
	Msg      Msger    `json:"msg"`      // Msg content, implemented via the interface for customization
	Extra    Extra    `json:"extra"`    // Extra content, implemented via the interface for customization
}

type Msger interface {
	MsgToString() (string, error)
}

type Extra interface {
	ExtraToString() (string, error)
}

type (
	// StdoutWriter writes logs to standard out.
	StdoutWriter struct{}

	// StderrWriter writes logs to standard error.
	StderrWriter struct{}

	// DiscardWriter discards all writes.
	DiscardWriter struct{}
)

// UniModule returns the Uni module information
func (StdoutWriter) UniModule() ModuleInfo {
	return ModuleInfo{
		ID: "uni.logging.writers.stdout",
		New: func() Module {
			return new(StdoutWriter)
		},
	}
}

// UniModule returns the Uni module information
func (StderrWriter) UniModule() ModuleInfo {
	return ModuleInfo{
		ID:  "uni.logging.writers.stderr",
		New: func() Module { return new(StderrWriter) },
	}
}

// UniModule returns the Uni module information
func (DiscardWriter) UniModule() ModuleInfo {
	return ModuleInfo{
		ID:  "uni.logging.writers.discard",
		New: func() Module { return new(DiscardWriter) },
	}
}

func (StdoutWriter) String() string  { return "stdout" }
func (StderrWriter) String() string  { return "stderr" }
func (DiscardWriter) String() string { return "discard" }

// WriterID returns a unique ID representing stdout.
func (StdoutWriter) WriterID() string { return "std:out" }

// WriterID returns a unique ID representing stderr.
func (StderrWriter) WriterID() string { return "std:err" }

// WriterID returns a unique key representing discard.
func (DiscardWriter) WriterID() string { return "discard" }

// OpenWriter returns os.Stdout that can't be closed.
func (StdoutWriter) OpenWriter() (io.WriteCloser, error) {
	return notClosable{os.Stdout}, nil
}

// OpenWriter returns io.Discard that can't be closed.
func (StderrWriter) OpenWriter() (io.WriteCloser, error) {
	return notClosable{os.Stderr}, nil
}

// OpenWriter returns io.Discard that can't be closed.
func (DiscardWriter) OpenWriter() (io.WriteCloser, error) {
	return notClosable{io.Discard}, nil
}

// notClosable is an io.WriteCloser that can't be closed.
type notClosable struct{ io.Writer }

func (fc notClosable) Close() error { return nil }

type defaultCustomLog struct {
	*CustomLog
	logger *zap.Logger
}

// newDefaultProductionLog configures a custom log that is
// intended for use by default if no other log is specified
// in a config.
//
// It writes to stderr, uses the console encoder,
// and enables INFO-level logs and higher.
func newDefaultProductionLog() (*defaultCustomLog, error) {
	cl := new(CustomLog)
	cl.writerFactory = StderrWriter{}
	var err error
	cl.writer, err = cl.writerFactory.OpenWriter()
	if err != nil {
		return nil, err
	}
	cl.encoder = newDefaultProductionLogEncoder(cl.writerFactory)
	cl.levelEnabler = zapcore.InfoLevel

	cl.buildCore()

	logger := zap.New(cl.core)

	// capture logs from other libraries which
	// may not be using zap logging directly
	_ = zap.RedirectStdLog(logger)

	return &defaultCustomLog{
		CustomLog: cl,
		logger:    logger,
	}, nil
}

func newDefaultProductionLogEncoder(wo WriterFactory) zapcore.Encoder {
	encCfg := zap.NewProductionEncoderConfig()
	if IsWriterStandardStream(wo) && term.IsTerminal(int(os.Stderr.Fd())) {
		encCfg.EncodeTime = func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
			pae.AppendString(t.UTC().Format("2006/01/02 15:04:05.000"))
		}
		if coloringEnabled {
			encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}
		return zapcore.NewConsoleEncoder(encCfg)
	}
	return zapcore.NewJSONEncoder(encCfg)
}

// BufferedLog sets the default logger to one that buffers
// logs before a config is loaded.
//
// Returns the buffered logger, the original default logger
// (for flushing on errors), and the buffer core so that the
// caller can flush the logs after the config is loaded or
// fails to load.
func BufferedLog() (*zap.Logger, *zap.Logger, *internal.LogBufferCore) {
	defaultLoggerMu.Lock()
	defer defaultLoggerMu.Unlock()
	origLogger := defaultLogger.logger
	bufferCore := internal.NewLogBufferCore(zap.InfoLevel)
	defaultLogger.logger = zap.New(bufferCore)
	return defaultLogger.logger, origLogger, bufferCore
}

var (
	defaultLoggerMu  sync.RWMutex
	defaultLogger, _ = newDefaultProductionLog()
	// enable color if NO_COLOR is not set and terminal is not xterm-mono
	coloringEnabled = os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "xterm-mono"
)
