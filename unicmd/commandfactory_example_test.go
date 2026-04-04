// Package unicmd provides a factory-based command framework for building CLI applications.
// This file contains usage examples for the RootCmdFactory.
package unicmd_test

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yonomesh/uni/unicmd"
)

// This example shows the basic usage of RootCmdFactory to create a CLI application.
func ExampleRootCmdFactory_basic() {
	// 1. Create a factory that defines the root command
	factory := unicmd.NewRootCmdFactory(func() *cobra.Command {
		return &cobra.Command{
			Use:   "myapp",
			Short: "My application built on uni framework",
		}
	})

	// 2. Register a "serve" command
	factory.RegisterCommand(unicmd.Command{
		Name:  "serve",
		Short: "Start the server",
		CobraFunc: func(cmd *cobra.Command) {
			cmd.RunE = unicmd.CommandFuncToCobraRunE(func(fl unicmd.Flags) (int, error) {
				fmt.Println("Server starting...")
				// In a real implementation, you would start your server here
				return 0, nil
			})
		},
	})

	// 3. Register a "version" command  
	factory.RegisterCommand(unicmd.Command{
		Name:  "version",
		Short: "Show version information",
		CobraFunc: func(cmd *cobra.Command) {
			cmd.RunE = unicmd.CommandFuncToCobraRunE(func(fl unicmd.Flags) (int, error) {
				fmt.Println("myapp v1.0.0")
				return 0, nil
			})
		},
	})

	// 4. Build the complete command tree
	rootCmd := factory.Build()

	// 5. Execute with "version" argument (for demonstration)
	rootCmd.SetArgs([]string{"version"})
	rootCmd.SetOut(os.Stdout)
	_ = rootCmd.Execute()
	// Output: myapp v1.0.0
}

// This example shows how to inspect registered commands using the Commands() method.
func ExampleRootCmdFactory_commands() {
	factory := unicmd.NewRootCmdFactory(func() *cobra.Command {
		return &cobra.Command{Use: "app"}
	})

	factory.RegisterCommand(unicmd.Command{
		Name:      "start",
		Short:     "Start the service",
		CobraFunc: func(cmd *cobra.Command) {},
	})

	factory.RegisterCommand(unicmd.Command{
		Name:      "stop", 
		Short:     "Stop the service",
		CobraFunc: func(cmd *cobra.Command) {},
	})

	// Get list of registered commands
	commands := factory.Commands()
	for _, cmd := range commands {
		fmt.Printf("%s: %s\n", cmd.Name, cmd.Short)
	}
	// Unordered output:
	// start: Start the service
	// stop: Stop the service
}
