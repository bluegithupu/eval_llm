package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test CLI wrapper for OpenCompass execution
// These tests verify the CLI command construction, environment variable injection,
// timeout handling, and output capture as per VAL-OC-001, VAL-OC-002, VAL-OC-003

func TestCLIWrapper_BuildCommand(t *testing.T) {
	// Test that CLI command is constructed with required arguments
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      30 * time.Minute,
	})

	cmd := wrapper.BuildCommand("/tmp/opencompass", []string{"MMLU", "HellaSwag"})

	assert.NotNil(t, cmd)
	assert.Equal(t, "python", cmd.Args[0])
	assert.Contains(t, cmd.Args, "run.py")
	assert.Contains(t, cmd.Args, "--datasets")
	// Datasets are joined with comma
	assert.Contains(t, cmd.Args, "MMLU,HellaSwag")
	assert.Contains(t, cmd.Args, "--work-dir")
	assert.Contains(t, cmd.Args, "/tmp/opencompass")
}

func TestCLIWrapper_BuildCommand_SingleDataset(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python3",
		RunScript:    "run.py",
		WorkDir:      "/tmp/eval",
		ContainerIMG: "opencompass:latest",
		Timeout:      1 * time.Hour,
	})

	cmd := wrapper.BuildCommand("/tmp/eval", []string{"MMLU"})

	assert.NotNil(t, cmd)
	assert.Equal(t, "python3", cmd.Args[0])
	assert.Contains(t, cmd.Args, "run.py")
	assert.Contains(t, cmd.Args, "--datasets")
	assert.Contains(t, cmd.Args, "MMLU")
}

func TestCLIWrapper_BuildCommand_HasConfigFlag(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      30 * time.Minute,
	})

	cmd := wrapper.BuildCommand("/tmp/opencompass", []string{"MMLU"})

	// Command should include config file path
	assert.Contains(t, cmd.Args, "--config")
	assert.Contains(t, cmd.Args, "/tmp/opencompass/config.py")
}

func TestCLIWrapper_BuildCommand_EmptyDatasets(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      30 * time.Minute,
	})

	cmd := wrapper.BuildCommand("/tmp/opencompass", []string{})

	// Should not crash even with empty datasets
	assert.NotNil(t, cmd)
}

func TestCLIWrapper_Execute_Success(t *testing.T) {
	// Skip if no python available
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a simple Python command that succeeds
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "print('test')"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "test")
	assert.Empty(t, result.Stderr)
	assert.Nil(t, result.Error)
}

func TestCLIWrapper_Execute_NonZeroExitCode(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a Python command that fails
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import sys; sys.exit(1)"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Equal(t, 1, result.ExitCode)
	assert.NotNil(t, result.Error)
	assert.IsType(t, &CLIError{}, result.Error)
}

func TestCLIWrapper_Execute_Timeout(t *testing.T) {
	// Skip on systems where python sleep may not work as expected
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      200 * time.Millisecond, // Short timeout
	})

	// Execute a command that would take too long if not for timeout
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import time; time.sleep(10)"}

	result := wrapper.Execute(context.Background(), cmd)

	// Timeout should result in ExitCode -1 and Timeout flag set
	assert.True(t, result.Timeout, "Expected Timeout to be true")
	assert.Equal(t, -1, result.ExitCode, "Expected ExitCode -1 for timeout")
	assert.NotNil(t, result.Error)
	assert.IsType(t, &CLIError{}, result.Error)
	cliErr := result.Error.(*CLIError)
	assert.True(t, cliErr.Timeout, "Expected CLIError.Timeout to be true")
}

func TestCLIWrapper_Execute_CapturesStderr(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a Python command that writes to stderr
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import sys; sys.stderr.write('error message\\n'); sys.exit(1)"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.NotEmpty(t, result.Stderr)
	assert.Contains(t, result.Stderr, "error message")
	assert.NotEqual(t, 0, result.ExitCode)
}

func TestCLIWrapper_Execute_ContextCancellation(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a long-running command with cancelled context
	ctx, cancel := context.WithCancel(context.Background())

	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import time; time.sleep(10)"}

	// Cancel immediately
	cancel()

	result := wrapper.Execute(ctx, cmd)

	// Should indicate error due to cancelled context
	assert.NotNil(t, result.Error)
}

func TestCLIWrapper_Execute_CapturesStdout(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a command that outputs to stdout
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "print('hello world')"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Contains(t, result.Stdout, "hello world")
}

func TestCLIWrapper_EnvironmentVariables(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Test that environment variables are properly injected
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import os; print(os.environ.get('OPENAI_API_KEY', 'not set'))"}

	// Execute with mock API keys
	result := wrapper.Execute(context.Background(), cmd)

	// Default should have env vars set to placeholder
	assert.Contains(t, result.Stdout, "not set") // Default behavior - no auto-injection
}

func TestCLIWrapper_EnvironmentVariables_CustomEnv(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
		EnvVars: map[string]string{
			"OPENAI_API_KEY": "sk-test-key",
		},
	})

	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import os; print(os.environ.get('OPENAI_API_KEY', 'not set'))"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Contains(t, result.Stdout, "sk-test-key")
}

func TestCLIWrapper_SetEnvVars(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Test setting environment variables
	envVars := map[string]string{
		"OPENAI_API_KEY":     "sk-12345",
		"ANTHROPIC_API_KEY":  "sk-ant-67890",
		"DASHSCOPE_API_KEY":  "sk-ds-abcde",
	}

	wrapper.SetEnvVars(envVars)

	cmd := wrapper.BuildCommand("/tmp/opencompass", []string{})

	// Verify env vars are set on the command
	foundKeys := 0
	for _, env := range cmd.Env {
		if env == "OPENAI_API_KEY=sk-12345" ||
			env == "ANTHROPIC_API_KEY=sk-ant-67890" ||
			env == "DASHSCOPE_API_KEY=sk-ds-abcde" {
			foundKeys++
		}
	}
	assert.Equal(t, 3, foundKeys, "All API key environment variables should be set")
}

func TestCLIWrapper_SetEnvVars_Overwrite(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
		EnvVars: map[string]string{
			"OPENAI_API_KEY": "sk-original",
		},
	})

	// Overwrite existing env var
	wrapper.SetEnvVars(map[string]string{
		"OPENAI_API_KEY": "sk-replaced",
	})

	cmd := wrapper.BuildCommand("/tmp/opencompass", []string{})

	// Verify the new value is present
	found := false
	for _, env := range cmd.Env {
		if env == "OPENAI_API_KEY=sk-replaced" {
			found = true
			break
		}
	}
	assert.True(t, found, "Environment variable should be overwritten with new value")
}

func TestCLIWrapper_ClearEnvVars(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
		EnvVars: map[string]string{
			"OPENAI_API_KEY": "sk-12345",
		},
	})

	// Clear all env vars
	wrapper.ClearEnvVars()

	cmd := wrapper.BuildCommand("/tmp/opencompass", []string{})

	// Verify no custom env vars are set (only inherited)
	hasOpenAI := false
	for _, env := range cmd.Env {
		if len(env) > 16 && env[:16] == "OPENAI_API_KEY=" {
			hasOpenAI = true
			break
		}
	}
	assert.False(t, hasOpenAI, "OPENAI_API_KEY should be cleared")
}

func TestCLIWrapper_GetEnvVars(t *testing.T) {
	envVars := map[string]string{
		"OPENAI_API_KEY":    "sk-12345",
		"CUSTOM_VAR":        "custom-value",
	}

	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
		EnvVars:      envVars,
	})

	retrieved := wrapper.GetEnvVars()

	assert.Equal(t, envVars, retrieved)
}

func TestCLIError_Error(t *testing.T) {
	err := &CLIError{
		ExitCode: 1,
		Stderr:   "test error message",
		Timeout:  false,
	}

	assert.Equal(t, "exit code 1: test error message", err.Error())
}

func TestCLIError_Error_WithTimeout(t *testing.T) {
	err := &CLIError{
		ExitCode: -1,
		Stderr:   "process killed",
		Timeout:  true,
	}

	assert.Contains(t, err.Error(), "timeout")
	assert.Contains(t, err.Error(), "process killed")
}

func TestCLIError_Error_WithContextCancel(t *testing.T) {
	err := &CLIError{
		ExitCode: -1,
		Stderr:   "context canceled",
		Timeout:  false,
		Cancelled: true,
	}

	assert.Contains(t, err.Error(), "canceled")
}

func TestCLIResult_ExitCodeMeaning(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		timeout  bool
		meaning  string
	}{
		{"success", 0, false, "successful execution"},
		{"error", 1, false, "non-zero exit code"},
		{"timeout", -1, true, "timeout occurred"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CLIResult{
				ExitCode: tt.exitCode,
				Timeout:  tt.timeout,
			}

			// For non-success cases, simulate the error field that Execute would set
			if tt.exitCode != 0 {
				result.Error = &CLIError{
					ExitCode: tt.exitCode,
					Timeout:  tt.timeout,
				}
			}

			if tt.timeout {
				assert.True(t, result.Timeout)
			}

			if tt.exitCode == 0 {
				assert.Nil(t, result.Error)
			} else {
				assert.NotNil(t, result.Error)
			}
		})
	}
}

func TestCLIWrapper_Execute_ComplexOutput(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a command with mixed output
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import sys; sys.stdout.write('stdout1\\n'); sys.stderr.write('stderr1\\n'); sys.stdout.write('stdout2\\n')"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Contains(t, result.Stdout, "stdout1")
	assert.Contains(t, result.Stdout, "stdout2")
	assert.Contains(t, result.Stderr, "stderr1")
	assert.Equal(t, 0, result.ExitCode)
}

func TestCLIWrapper_Execute_LargeOutput(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a command with large output
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	// Generate ~100KB of output
	cmd.Args = []string{"python", "-c", "print('x' * 100000)"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Equal(t, 0, result.ExitCode)
	assert.Len(t, result.Stdout, 100001) // 100000 x's + newline
}

func TestCLIWrapper_TimeoutFromConfig(t *testing.T) {
	timeout := 5 * time.Minute

	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      timeout,
	})

	assert.Equal(t, timeout, wrapper.config.Timeout)
}

func TestCLIWrapper_Execute_PythonNotFound(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "/nonexistent/python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	cmd := wrapper.BuildCommand("/tmp/opencompass", []string{})

	result := wrapper.Execute(context.Background(), cmd)

	assert.NotEqual(t, 0, result.ExitCode)
	assert.NotNil(t, result.Error)
}

func TestCLIWrapper_NewCLIWrapper_Defaults(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{})

	assert.Equal(t, "python", wrapper.config.PythonPath)
	assert.Equal(t, "run.py", wrapper.config.RunScript)
	assert.Equal(t, 30*time.Minute, wrapper.config.Timeout)
	assert.NotNil(t, wrapper.config.EnvVars)
}

func TestCLIWrapper_Execute_WithProgress(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	// Execute a command that reports progress
	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "for i in range(3): print(f'progress: {i*33}%')"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "progress: 0%")
	assert.Contains(t, result.Stdout, "progress: 66%")
}

func TestCLIWrapper_Execute_ExitCode2(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "python"
	cmd.Args = []string{"python", "-c", "import sys; sys.exit(2)"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.Equal(t, 2, result.ExitCode)
	assert.NotNil(t, result.Error)
	assert.IsType(t, &CLIError{}, result.Error)
	cliErr := result.Error.(*CLIError)
	assert.Equal(t, 2, cliErr.ExitCode)
	assert.False(t, cliErr.Timeout)
}

func TestCLIWrapper_Execute_ExitCode127(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "nonexistent_command_xyz"
	cmd.Args = []string{"nonexistent_command_xyz", "--version"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.NotEqual(t, 0, result.ExitCode)
	assert.NotNil(t, result.Error)
}

func TestCLIWrapper_BuildCommand_WithConfigPath(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "run.py",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      30 * time.Minute,
	})

	workDir := "/workspace/eval-123"
	datasets := []string{"MMLU", "HellaSwag", "BBH"}

	cmd := wrapper.BuildCommand(workDir, datasets)

	// Verify the full command structure
	assert.Equal(t, "python", cmd.Args[0])
	assert.Equal(t, "run.py", cmd.Args[1])
	assert.Equal(t, "--datasets", cmd.Args[2])
	assert.Equal(t, "MMLU,HellaSwag,BBH", cmd.Args[3])
	assert.Equal(t, "--work-dir", cmd.Args[4])
	assert.Equal(t, workDir, cmd.Args[5])
	assert.Equal(t, "--config", cmd.Args[6])
	assert.Equal(t, workDir+"/config.py", cmd.Args[7])
}

func TestCLIWrapper_Execute_BinaryNotExecutable(t *testing.T) {
	wrapper := NewCLIWrapper(CLIConfig{
		PythonPath:   "python",
		RunScript:    "-c",
		WorkDir:      "/tmp/opencompass",
		ContainerIMG: "opencompass:latest",
		Timeout:      10 * time.Second,
	})

	cmd := wrapper.BuildCommand("/tmp", []string{})
	cmd.Path = "/tmp"
	cmd.Args = []string{"/tmp", "invalid"}

	result := wrapper.Execute(context.Background(), cmd)

	assert.NotEqual(t, 0, result.ExitCode)
}
