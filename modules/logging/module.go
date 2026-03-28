package logging

import "github.com/yonomesh/uni"

// DelegateSetDefaultFormatForWriter allows an encoder (especially wrapp
// a encoder) to configure or delegate default format selection once
// the final output writer is known at runtime.
type DelegateSetDefaultFormatForWriter interface {
	// SetWriterDefaultFormat configures the default format of the encoder
	// (usually the wrapped encoder) according to the provided WriterProvider.
	//
	// The writer parameter allows the encoder to make runtime decisions,
	// such as switching to a console-friendly format if the output is a terminal.
	SetWriterDefaultFormat(wp uni.WriterProvider) error
}

const DefaultLoggerName = "default"
