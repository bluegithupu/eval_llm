# OpenCompass CLI Integration Behavioral Assertions

This document enumerates all behavioral assertions for the OpenCompass LLM evaluation backend integration.

---

## CLI Execution

### VAL-OC-001: CLI Command Construction with Required Arguments
The system shall construct the CLI command with the correct Python interpreter and run.py entry point, including all required arguments (--datasets, --hf-path, --work-dir) in the correct order.

**Pass condition:** Command string contains `python run.py` with all required arguments present and properly formatted.

**Tool:** subprocess

**Evidence:** Command string logged before execution; full command line with arguments.

---

### VAL-OC-002: Subprocess Invocation with Correct Working Directory
The system shall invoke the OpenCompass CLI subprocess from the correct working directory where OpenCompass is installed, ensuring run.py is accessible.

**Pass condition:** Subprocess executes from the OpenCompass installation directory; FileNotFoundError is not raised for run.py.

**Tool:** subprocess

**Evidence:** Subprocess stdout/stderr logs; working directory path recorded.

---

### VAL-OC-003: Environment Variables Propagation for CUDA
The system shall set CUDA_VISIBLE_DEVICES environment variable to control GPU allocation for the subprocess, propagating the specified GPU IDs.

**Pass condition:** CUDA_VISIBLE_DEVICES environment variable is set in subprocess environment; value matches specified GPU configuration.

**Tool:** subprocess

**Evidence:** Environment dictionary logged before subprocess call; GPU allocation in output logs.

---

### VAL-OC-004: Environment Variables for API Keys
The system shall inject API keys (OPENAI_API_KEY, ANTHROPIC_API_KEY, DASHSCOPE_API_KEY) as environment variables into the subprocess for API model authentication.

**Pass condition:** Relevant API key environment variable is set when evaluating API models; key is not exposed in command-line arguments.

**Tool:** subprocess

**Evidence:** Environment dictionary (with keys redacted in logs); successful API authentication in output.

---

### VAL-OC-005: Subprocess Timeout Handling
The system shall enforce a configurable timeout on the subprocess execution, terminating the process if evaluation exceeds the timeout threshold.

**Pass condition:** Long-running evaluations are terminated after timeout; TimeoutExpired exception is caught and handled gracefully.

**Tool:** subprocess

**Evidence:** Timeout exception logs; process termination recorded; error message returned to caller.

---

### VAL-OC-006: Subprocess Output Capture
The system shall capture both stdout and stderr from the subprocess, preserving the complete output for logging and error analysis.

**Pass condition:** Both stdout and stderr are captured as strings; output is available after subprocess completion regardless of exit code.

**Tool:** subprocess

**Evidence:** Captured stdout and stderr content; output stored in run logs.

---

## Configuration File Generation

### VAL-OC-007: Valid Python Configuration File Syntax
The system shall generate MMEngine-compatible Python configuration files with valid syntax, using correct imports and dictionary structures.

**Pass condition:** Generated config file is valid Python; can be loaded with `exec()` or MMEngine Config.fromfile() without SyntaxError.

**Tool:** subprocess

**Evidence:** Generated config file content; successful config load in logs.

---

### VAL-OC-008: Model Configuration Required Fields
The system shall include all required fields in model configuration: type, path, max_seq_len, max_out_len, batch_size, and run_cfg.

**Pass condition:** Generated model config contains all required fields; missing fields cause validation error before execution.

**Tool:** subprocess

**Evidence:** Generated config file; field validation output.

---

### VAL-OC-009: Dataset Configuration with Correct Inferencer
The system shall generate dataset configurations with the appropriate inferencer type (PPLInferencer for multiple-choice, GenInferencer for generative tasks) matching the dataset requirements.

**Pass condition:** Dataset config specifies correct inferencer type; MMLU uses PPLInferencer, generative benchmarks use GenInferencer.

**Tool:** subprocess

**Evidence:** Generated dataset config section; successful dataset evaluation logs.

---

### VAL-OC-010: MMLU Dataset Configuration
The system shall generate valid MMLU dataset configuration with correct prompt template, reader configuration, and evaluation settings.

**Pass condition:** MMLU dataset config includes input_columns, output_column, test_split, and AccEvaluator.

**Tool:** subprocess

**Evidence:** Generated MMLU config; evaluation output showing MMLU scores.

---

### VAL-OC-011: HellaSwag Dataset Configuration
The system shall generate valid HellaSwag dataset configuration with appropriate inferencer (PPLInferencer) and evaluation metrics.

**Pass condition:** HellaSwag config specifies correct prompt template for the 4-way choice format; evaluation returns accuracy score.

**Tool:** subprocess

**Evidence:** Generated HellaSwag config; evaluation output showing HellaSwag scores.

---

## API Model Configuration

### VAL-OC-012: OpenAI Model Configuration
The system shall generate OpenAI model configuration with type=OpenAI, path='gpt-4' (or specified model), key sourced from OPENAI_API_KEY environment variable, and run_cfg with num_gpus=0.

**Pass condition:** Model config has type=OpenAI; path matches specified model; key references environment variable; num_gpus is 0.

**Tool:** subprocess

**Evidence:** Generated OpenAI model config section; successful API call in logs.

---

### VAL-OC-013: Claude Model Configuration
The system shall generate Claude model configuration with correct type (ZhiPuAI or Anthropic-based), key from ANTHROPIC_API_KEY environment variable, and appropriate max_seq_len for Claude context window.

**Pass condition:** Claude model config uses correct class type; key sourced from environment; max_seq_len within Claude limits.

**Tool:** subprocess

**Evidence:** Generated Claude model config; successful Claude API call logs.

---

### VAL-OC-014: Qwen Model Configuration
The system shall generate Qwen model configuration with type referencing DashScope API, key from DASHSCOPE_API_KEY environment variable, and model path matching Qwen model identifier.

**Pass condition:** Qwen config uses DashScope-compatible type; key sourced from DASHSCOPE_API_KEY; model path is valid Qwen identifier.

**Tool:** subprocess

**Evidence:** Generated Qwen model config; successful Qwen API call logs.

---

### VAL-OC-015: API Key Not in Command Line Arguments
The system shall NEVER pass API keys as command-line arguments; all keys must be injected via environment variables only.

**Pass condition:** Command string does not contain API key values; keys only appear in subprocess environment dictionary.

**Tool:** subprocess

**Evidence:** Command string logged; environment dictionary (keys redacted).

---

## Result Collection

### VAL-OC-016: JSON Predictions Parsing
The system shall parse JSON prediction files from the predictions/ directory, extracting question, prediction, and answer fields for each sample.

**Pass condition:** JSON files are loaded successfully; prediction list contains expected fields; no JSONDecodeError.

**Tool:** subprocess

**Evidence:** Parsed prediction data structure; sample count matches expected.

---

### VAL-OC-017: CSV Summary Extraction
The system shall extract the summary CSV from the output directory, parsing dataset names, model names, and accuracy scores.

**Pass condition:** summary.csv exists in output directory; CSV is parsed into structured format; contains all evaluated datasets.

**Tool:** subprocess

**Evidence:** Parsed CSV content; extracted scores dictionary.

---

### VAL-OC-018: Results Directory Structure Validation
The system shall validate that the output directory contains the expected structure: predictions/, results/, summary/, and logs/ subdirectories.

**Pass condition:** All expected subdirectories exist; missing directories are reported as errors.

**Tool:** subprocess

**Evidence:** Directory listing; validation result logged.

---

## Output Directory Management

### VAL-OC-019: Timestamp-Based Output Directory Naming
The system shall create output directories with timestamp-based naming (YYYYMMDD_HHMMSS format) under the specified work directory.

**Pass condition:** Output directory name matches timestamp format; directory is created at evaluation start.

**Tool:** subprocess

**Evidence:** Output directory path; creation timestamp logged.

---

### VAL-OC-020: Unique Output Directory Per Evaluation
The system shall create a unique output directory for each evaluation run, ensuring results from different runs do not overwrite each other.

**Pass condition:** Each evaluation creates new directory; no collision with existing directories.

**Tool:** subprocess

**Evidence:** Multiple evaluation output directories; unique naming verified.

---

### VAL-OC-021: Output File Naming Convention
The system shall use consistent file naming for prediction and result files following OpenCompass conventions: {dataset}/{model}.json for results.

**Pass condition:** Result files follow naming convention; files are discoverable by expected paths.

**Tool:** subprocess

**Evidence:** Generated file paths; file listing in output directory.

---

### VAL-OC-022: Output Directory Cleanup After Collection
The system shall clean up (remove) output directory contents after successful result collection when cleanup is enabled, preserving disk space.

**Pass condition:** Output directory is removed or emptied after results are extracted; cleanup flag respected.

**Tool:** subprocess

**Evidence:** Directory existence check after cleanup; cleanup operation logged.

---

### VAL-OC-023: Partial Result Preservation on Failure
The system shall preserve output directory on evaluation failure to allow debugging, only cleaning up on successful completion.

**Pass condition:** Output directory remains when subprocess returns non-zero; cleanup is skipped on error.

**Tool:** subprocess

**Evidence:** Output directory existence check after failed run; error logs preserved.

---

## Error Handling

### VAL-OC-024: Non-Zero Exit Code Capture
The system shall capture and report subprocess non-zero exit codes, distinguishing between different failure types (1 for errors, 137 for OOM, etc.).

**Pass condition:** Non-zero exit code is captured; error type is logged; failure is reported to caller.

**Tool:** subprocess

**Evidence:** Exit code in result dictionary; stderr captured; error message returned.

---

### VAL-OC-025: Stderr Error Logging
The system shall log the complete stderr output from failed subprocess execution for debugging purposes.

**Pass condition:** Stderr is captured and logged; error context is preserved; logs are accessible for debugging.

**Tool:** subprocess

**Evidence:** Stderr content in logs; timestamp of failure recorded.

---

### VAL-OC-026: Invalid Output Format Handling
The system shall handle cases where output files are malformed or missing expected fields, reporting structured errors without crashing.

**Pass condition:** Malformed JSON/CSV causes controlled error; error message indicates which file/field is invalid; system remains stable.

**Tool:** subprocess

**Evidence:** Error logs for malformed files; exception handling verified.

---

### VAL-OC-027: Missing Output Directory Handling
The system shall handle cases where the expected output directory is not created (evaluation crashed before completion), reporting a clear error.

**Pass condition:** Missing directory is detected; FileNotFoundError or equivalent error is caught; descriptive error is returned.

**Tool:** subprocess

**Evidence:** Error message for missing directory; graceful failure handling.

---

### VAL-OC-028: GPU Memory Error Detection
The system shall detect GPU out-of-memory errors from stderr/stdout and report them distinctly from other failure types.

**Pass condition:** OOM error patterns (CUDA out of memory, RuntimeError) are detected; specific OOM error is returned.

**Tool:** subprocess

**Evidence:** OOM detection in logs; distinct error type returned.

---

### VAL-OC-029: API Rate Limit Error Handling
The system shall detect API rate limit errors from OpenAI/Claude APIs and report them with retry guidance.

**Pass condition:** Rate limit error messages (429 status, rate limit exceeded) are detected; error includes retry suggestion.

**Tool:** subprocess

**Evidence:** Rate limit error logs; structured error response.

---

### VAL-OC-030: Configuration Syntax Error Detection
The system shall validate generated configuration files for Python syntax errors before subprocess execution, failing fast with clear error message.

**Pass condition:** Invalid config syntax is caught before subprocess; error indicates line number and syntax issue.

**Tool:** subprocess

**Evidence:** Syntax error validation logs; pre-execution failure message.

---

## Summary

| Category | Assertion Count |
|----------|-----------------|
| CLI Execution | 6 |
| Configuration Generation | 5 |
| API Model Setup | 4 |
| Result Collection | 3 |
| Output Management | 5 |
| Error Handling | 7 |
| **Total** | **30** |

---

## Evidence Collection Template

For each test execution, collect:

1. **Command String**: Full CLI command with arguments (redact API keys)
2. **Environment Dictionary**: Keys set for subprocess (redact values)
3. **Generated Config File**: Complete config.py content
4. **Subprocess Return Code**: Exit code integer
5. **Stdout Content**: Complete stdout capture
6. **Stderr Content**: Complete stderr capture
7. **Output Directory Listing**: File tree structure
8. **Parsed Results**: JSON predictions and CSV summary data
9. **Timestamps**: Start time, end time, duration
10. **Resource Metrics**: GPU usage, memory consumption (if available)
