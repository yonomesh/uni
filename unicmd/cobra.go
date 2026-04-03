package unicmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func UniCmdToCobra(uniCmd Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   uniCmd.Name + " " + uniCmd.Usage,
		Short: uniCmd.Short,
		Long:  uniCmd.Long,
	}

	uniCmd.CobraFunc(cmd)

	return cmd
}

// CommandFuncToCobraRunE wraps a Uni CommandFunc for use
// in a cobra command's RunE field.
func CommandFuncToCobraRunE(f CommandFunc) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		// wrap cobra flags → uni flags, then execute command
		status, err := f(Flags{cmd.Flags()}) // key point
		if status > 1 {
			cmd.SilenceErrors = true
			return &ExitError{ExitCode: status, Err: err}
		}
		return err
	}
}

// exitError carries the exit code from CommandFunc to Main()
type ExitError struct {
	ExitCode int
	Err      error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exiting with code %d", e.ExitCode)
	}
	return e.Err.Error()
}
