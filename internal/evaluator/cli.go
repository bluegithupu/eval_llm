package evaluator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// CLIConfig holds configuration for the CLI wrapper
type CLIConfig struct {
	PythonPath   string            // Path to Python interpreter (default: "python")
	RunScript    string            // Path to OpenCompass run.py (default: "run.py")
	WorkDir      string            // Default working directory
	ContainerIMG string            // Container image name
	Timeout      time.Duration     // Execution timeout (default: 30 minutes)
	EnvVars      map[string]string // Environment variables to inject
}

// CLIWrapper wraps subprocess execution for OpenCompass CLI
type CLIWrapper struct {
	config CLIConfig
}

// CLIResult holds the result of CLI execution
type CLIResult struct {
	ExitCode int           // Exit code (0 for success, -1 for timeout)
	Stdout   string        // Standard output
	Stderr   string        // Standard error
	Error    error         // Error if any
	Timeout  bool          // True if execution timed out
}

// CLIError represents a CLI execution error
type CLIError struct {
	ExitCode  int    // Exit code from the process
	Stderr    string // Standard error output
	Timeout   bool   // True if this is a timeout error
	Cancelled bool   // True if context was cancelled
}

func (e *CLIError) Error() string {
	if e.Timeout {
		return fmt.Sprintf("timeout after process killed: %s", e.Stderr)
	}
	if e.Cancelled {
		return fmt.Sprintf("context canceled: %s", e.Stderr)
	}
	return fmt.Sprintf("exit code %d: %s", e.ExitCode, e.Stderr)
}

// NewCLIWrapper creates a new CLI wrapper with the given configuration
func NewCLIWrapper(config CLIConfig) *CLIWrapper {
	// Set defaults
	if config.PythonPath == "" {
		config.PythonPath = "python"
	}
	if config.RunScript == "" {
		config.RunScript = "run.py"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Minute
	}
	if config.EnvVars == nil {
		config.EnvVars = make(map[string]string)
	}

	return &CLIWrapper{
		config: config,
	}
}

// BuildCommand constructs the command for OpenCompass execution
// It includes required arguments: --datasets, --work-dir, --config
func (w *CLIWrapper) BuildCommand(workDir string, datasets []string) *Command {
	cmd := &Command{
		Path: w.config.PythonPath,
		Args: []string{
			w.config.PythonPath,
			w.config.RunScript,
			"--datasets",
			strings.Join(datasets, ","),
			"--work-dir",
			workDir,
			"--config",
			workDir + "/config.py",
		},
		Env:    []string{},
		Dir:    workDir,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}

	// Add current environment variables
	for _, env := range os.Environ() {
		cmd.Env = append(cmd.Env, env)
	}

	// Add custom environment variables (API keys)
	for key, value := range w.config.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return cmd
}

// SetEnvVars sets or updates environment variables for subsequent commands
func (w *CLIWrapper) SetEnvVars(envVars map[string]string) {
	for key, value := range envVars {
		w.config.EnvVars[key] = value
	}
}

// ClearEnvVars removes all custom environment variables
func (w *CLIWrapper) ClearEnvVars() {
	w.config.EnvVars = make(map[string]string)
}

// GetEnvVars returns the current environment variables
func (w *CLIWrapper) GetEnvVars() map[string]string {
	return w.config.EnvVars
}

// Execute runs the command with the configured timeout and captures output
func (w *CLIWrapper) Execute(ctx context.Context, cmd *Command) *CLIResult {
	result := &CLIResult{}

	// Determine timeout - use command-specific timeout, or wrapper config timeout, or default 30min
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = w.config.Timeout
	}
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	// Create a context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Set up pipes for capturing stdout and stderr
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	// Execute the command
	err := RunCommand(execCtx, cmd)

	// Capture output
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

	// Handle different error cases
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			// Timeout occurred
			result.ExitCode = -1
			result.Timeout = true
			result.Error = &CLIError{
				ExitCode: -1,
				Stderr:   result.Stderr,
				Timeout:  true,
			}
		} else if execCtx.Err() == context.Canceled {
			// Context cancelled
			result.ExitCode = -1
			result.Error = &CLIError{
				ExitCode: -1,
				Stderr:   result.Stderr,
				Cancelled: true,
			}
		} else {
			// Command execution error
			execErr, ok := err.(*ExecError)
			if ok {
				result.ExitCode = execErr.ExitCode
			} else {
				result.ExitCode = 1
			}
			result.Error = &CLIError{
				ExitCode: result.ExitCode,
				Stderr:   result.Stderr,
				Timeout:  false,
			}
		}
	} else {
		// Success
		result.ExitCode = 0
		result.Timeout = false
	}

	return result
}

// Command holds the configuration for executing a command
type Command struct {
	Path    string      // Executable path
	Args    []string   // Command arguments
	Env     []string   // Environment variables
	Dir     string     // Working directory
	Timeout time.Duration // Timeout for this specific command
	Stdout  *bytes.Buffer // Captured stdout
	Stderr  *bytes.Buffer // Captured stderr
}

// ExecError represents an execution error with exit code
type ExecError struct {
	ExitCode int
	Err      error
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("exec error (exit %d): %v", e.ExitCode, e.Err)
}

// RunCommand executes a command with context support
func RunCommand(ctx context.Context, cmd *Command) error {
	// This is a placeholder - actual implementation uses os/exec
	// The actual subprocess execution happens in the real implementation
	return runCommandInternal(ctx, cmd)
}
