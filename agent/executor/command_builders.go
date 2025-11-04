package executor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lureiny/lookingglass/agent/config"
	pb "github.com/lureiny/lookingglass/pb"
)

// BuildPingArgs builds ping command arguments from parameters
func BuildPingArgs(params *pb.NetworkTestParams) []string {
	args := make([]string, 0)

	// Count
	if params.Count > 0 {
		args = append(args, "-c", strconv.Itoa(int(params.Count)))
	}

	// Timeout (wait time for each packet)
	if params.Timeout > 0 {
		args = append(args, "-W", strconv.Itoa(int(params.Timeout)))
	}

	// IPv6
	if params.Ipv6 {
		args = append(args, "-6")
	} else {
		args = append(args, "-4")
	}

	// Target (must be last)
	args = append(args, params.Target)

	return args
}

// BuildMTRArgs builds MTR command arguments from parameters
func BuildMTRArgs(params *pb.NetworkTestParams) []string {
	args := make([]string, 0)

	// Report mode (non-interactive)
	args = append(args, "--report")

	// Count (number of pings to send)
	if params.Count > 0 {
		args = append(args, "--report-cycles", strconv.Itoa(int(params.Count)))
	} else {
		args = append(args, "--report-cycles", "10") // Default to 10
	}

	// No DNS resolution for faster results
	args = append(args, "--no-dns")

	// IPv6
	if params.Ipv6 {
		args = append(args, "-6")
	} else {
		args = append(args, "-4")
	}

	// Wide report (better formatting)
	args = append(args, "--report-wide")

	// Target (must be last)
	args = append(args, params.Target)

	return args
}

// BuildNextTraceArgs builds nexttrace command arguments from parameters
func BuildNextTraceArgs(params *pb.NetworkTestParams) []string {
	args := []string{}

	// IPv6 support
	if params.Ipv6 {
		args = append(args, "-6")
	} else {
		args = append(args, "-4")
	}

	// Max hops (similar to count in traceroute)
	if params.Count > 0 {
		args = append(args, "-m", strconv.Itoa(int(params.Count)))
	}

	// Timeout
	if params.Timeout > 0 {
		args = append(args, "-t", strconv.Itoa(int(params.Timeout)))
	}

	// Extra options from the map
	for key, value := range params.ExtraOptions {
		if value == "" {
			// Flag without value
			args = append(args, key)
		} else {
			args = append(args, key, value)
		}
	}

	// Add target at the end
	args = append(args, params.Target)

	return args
}

// AppendNewline is a line formatter that adds a newline to each line
func AppendNewline(line string) string {
	return line + "\n"
}

// BuildCustomCommandArgs builds custom command arguments with template support
// Supports placeholders: {target}, {count}, {timeout}, {ipv6}
func BuildCustomCommandArgs(defaultArgs []string, params *pb.NetworkTestParams) []string {
	if params == nil {
		return defaultArgs
	}

	args := make([]string, len(defaultArgs))
	copy(args, defaultArgs)

	// Template replacements
	replacements := map[string]string{
		"{target}":  params.Target,
		"{count}":   strconv.Itoa(int(params.Count)),
		"{timeout}": strconv.Itoa(int(params.Timeout)),
		"{ipv6}":    strconv.FormatBool(params.Ipv6),
	}

	// Apply replacements to each argument
	for i, arg := range args {
		for placeholder, value := range replacements {
			if strings.Contains(arg, placeholder) {
				args[i] = strings.ReplaceAll(arg, placeholder, value)
			}
		}
	}

	return args
}

// CreateCustomArgsBuilder creates an ArgsBuilder for custom commands
// Returns a function that applies template replacements to default args
func CreateCustomArgsBuilder(defaultArgs []string) ArgsBuilder {
	return func(params *pb.NetworkTestParams) []string {
		return BuildCustomCommandArgs(defaultArgs, params)
	}
}

// NewPingExecutor creates a new ping executor
func NewPingExecutor(pingPath string) *CommandExecutor {
	if pingPath == "" {
		pingPath = "/bin/ping" // Default path
	}
	return NewCommandExecutor(
		"ping",
		pingPath,
		BuildPingArgs,
		nil, // No line formatter needed
	)
}

// NewMTRExecutor creates a new MTR executor
func NewMTRExecutor(mtrPath string) *CommandExecutor {
	if mtrPath == "" {
		mtrPath = "/usr/bin/mtr" // Default path
	}
	return NewCommandExecutor(
		"MTR",
		mtrPath,
		BuildMTRArgs,
		nil, // No line formatter needed
	)
}

// NewNextTraceExecutor creates a new nexttrace executor
func NewNextTraceExecutor(nexttracePath string) *CommandExecutor {
	if nexttracePath == "" {
		nexttracePath = "/usr/bin/nexttrace" // Default path
	}
	return NewCommandExecutor(
		"nexttrace",
		nexttracePath,
		BuildNextTraceArgs,
		AppendNewline, // NextTrace needs newline appended
	)
}

// NewCustomCommandExecutor creates a custom command executor
// Parameters:
//   - name: Display name for the executor
//   - cmdPath: Path to the executable
//   - defaultArgs: Default arguments (supports template placeholders)
//   - needsNewline: Whether to append newline to each output line
func NewCustomCommandExecutor(name, cmdPath string, defaultArgs []string, needsNewline bool) *CommandExecutor {
	var lineFormatter LineFormatter
	if needsNewline {
		lineFormatter = AppendNewline
	}

	return NewCommandExecutor(
		name,
		cmdPath,
		CreateCustomArgsBuilder(defaultArgs),
		lineFormatter,
	)
}

// Factory functions for executor registry

// PingExecutorFactory creates a ping executor from configuration
func PingExecutorFactory(cfg *config.TaskConfig) (Executor, error) {
	path := "/bin/ping"
	if cfg.Executor != nil && cfg.Executor.Path != "" {
		path = cfg.Executor.Path
	}
	return NewPingExecutor(path), nil
}

// MTRExecutorFactory creates an MTR executor from configuration
func MTRExecutorFactory(cfg *config.TaskConfig) (Executor, error) {
	path := "/usr/bin/mtr"
	if cfg.Executor != nil && cfg.Executor.Path != "" {
		path = cfg.Executor.Path
	}
	return NewMTRExecutor(path), nil
}

// NextTraceExecutorFactory creates a nexttrace executor from configuration
func NextTraceExecutorFactory(cfg *config.TaskConfig) (Executor, error) {
	path := "/usr/bin/nexttrace"
	if cfg.Executor != nil && cfg.Executor.Path != "" {
		path = cfg.Executor.Path
	}
	return NewNextTraceExecutor(path), nil
}

// CommandExecutorFactory creates a custom command executor from configuration
func CommandExecutorFactory(cfg *config.TaskConfig) (Executor, error) {
	if cfg.Executor == nil {
		return nil, fmt.Errorf("executor configuration is required for command executor")
	}

	if cfg.Executor.Path == "" {
		return nil, fmt.Errorf("executor path is required for command executor")
	}

	needsNewline := cfg.Executor.LineFormatter == "newline"

	return NewCustomCommandExecutor(
		cfg.DisplayName,
		cfg.Executor.Path,
		cfg.Executor.DefaultArgs,
		needsNewline,
	), nil
}

// init registers all builtin executor factories
func init() {
	RegisterGlobal("ping", PingExecutorFactory)
	RegisterGlobal("mtr", MTRExecutorFactory)
	RegisterGlobal("nexttrace", NextTraceExecutorFactory)
	RegisterGlobal("command", CommandExecutorFactory)
}
