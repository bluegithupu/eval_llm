package evaluator

import (
	"context"
	"os/exec"
)

// runCommandInternal executes the command using os/exec
func runCommandInternal(ctx context.Context, cmd *Command) error {
	// Build the exec.Cmd
	execCmd := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	// Set working directory
	if cmd.Dir != "" {
		execCmd.Dir = cmd.Dir
	}

	// Set environment variables
	if len(cmd.Env) > 0 {
		execCmd.Env = cmd.Env
	}

	// Set up stdout capture
	if cmd.Stdout != nil {
		execCmd.Stdout = cmd.Stdout
	}

	// Set up stderr capture
	if cmd.Stderr != nil {
		execCmd.Stderr = cmd.Stderr
	}

	// Execute the command
	err := execCmd.Run()

	if err != nil {
		// Check if it's an exit error
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExecError{
				ExitCode: exitErr.ExitCode(),
				Err:      err,
			}
		}
		return err
	}

	return nil
}
