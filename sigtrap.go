package uni

import (
	"context"
	"os"
	"os/signal"

	"go.uber.org/zap"
)

func TrapSignals() {
	trapSignalsCrossPlatform()

}

// Double Check
// trapSignalsCrossPlatform captures SIGINT or interrupt (depending
// on the OS), which initiates a graceful shutdown. A second SIGINT
// or interrupt will forcefully exit the process immediately.
func trapSignalsCrossPlatform() {
	go func() {
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, os.Interrupt)

		<-shutdown
		// TODO Log().Info("shutting down", zap.String("signal", "SIGINT"))
		go exitProcessFromSignal("SIGINT")

		<-shutdown
		// TODO Log().Warn("force quit", zap.String("signal", "SIGINT"))
		os.Exit(ExitCodeForceQuit)

	}()
}

// exitProcessFromSignal exits the process from a system signal.
func exitProcessFromSignal(sigName string) {
	_ = sigName
	// TODO logger := Log().With(zap.String("signal", sigName))
	logger := &zap.Logger{}
	// TODO exitProcess
	exitProcess(context.TODO(), logger)
}

// Exit codes. Generally, you should NOT
// automatically restart the process if the
// exit code is ExitCodeFailedStartup (1).
//
// TODO
// #define	LINUX_REBOOT_MAGIC1	0xfee1dead
// 0xC0FFEE coffee
// 0x0BADC0DE bad code
const (
	ExitCodeSuccess = iota
	ExitCodeFailedStartup
	ExitCodeForceQuit
	ExitCodeFailedQuit
)
