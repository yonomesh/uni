package unicmd

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yonomesh/uni"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/caddyserver/certmagic"
	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

func setResourceLimits(logger *zap.Logger) func() {
	// Configure the maximum number of CPUs to use to match the Linux container quota (if any)
	// See https://pkg.go.dev/runtime#GOMAXPROCS
	undo, err := maxprocs.Set(maxprocs.Logger(logger.Sugar().Infof))
	if err != nil {
		logger.Warn("failed to set GOMAXPROCS", zap.Error(err))
	}

	// Configure the maximum memory to use to match the Linux container quota (if any) or system memory
	// See https://pkg.go.dev/runtime/debug#SetMemoryLimit
	_, _ = memlimit.SetGoMemLimitWithOpts(
		memlimit.WithLogger(
			slog.New(zapslog.NewHandler(logger.Core())),
		),
		memlimit.WithProvider(
			memlimit.ApplyFallback(
				memlimit.FromCgroup,
				memlimit.FromSystem,
			),
		),
	)

	return undo
}

// TODO
// LoadConfig loads the config from configFile it using adapterName.
// If no configFile is specified, it tries loading a default config file.
// The lack of a config file is not treated as an error, but false will be
// returned if there is no config available. It prints any warnings to stderr,
// and returns the resulting JSON config bytes along with the name of the
// loaded config file (if any).
// The return values are:
//   - config bytes (nil if no config)
//   - config file used ("" if none)
//   - error, if any
func LoadConfig(configFile string) ([]byte, string, error) {
	return []byte{}, "", nil
}

// handleEnvFileFlag loads the environment variables from the given --envfile
// flag if specified. This should be called as early in the command function.
func handleEnvFileFlag(fl Flags) error {
	var err error
	var envfileFlag []string
	envfileFlag, err = fl.GetStringSlice("envfile")
	if err != nil {
		return fmt.Errorf("reading envfile flag: %v", err)
	}

	for _, envfile := range envfileFlag {
		if err := loadEnvFromFile(envfile); err != nil {
			return fmt.Errorf("loading additional environment variables: %v", err)
		}
	}

	return nil
}

func loadEnvFromFile(envFile string) error {
	file, err := os.Open(envFile)
	if err != nil {
		return fmt.Errorf("reading environment file: %v", err)
	}
	defer file.Close()

	envMap, err := parseEnvFile(file)
	if err != nil {
		return fmt.Errorf("parsing environment file: %v", err)
	}

	for k, v := range envMap {
		// do not overwrite existing environment variables
		_, exists := os.LookupEnv(k)
		if !exists {
			if err := os.Setenv(k, v); err != nil {
				return fmt.Errorf("setting environment variables: %v", err)
			}
		}
	}

	// Update the storage paths to ensure they have the proper
	// value after loading a specified env file.
	uni.ConfigAutosavePath = filepath.Join(uni.AppConfigDir(), "autosave.json")
	uni.DefaultStorage = &certmagic.FileStorage{Path: uni.AppDataDir()}

	return nil
}

// parseEnvFile parses an env file from KEY=VALUE format.
// It's pretty naive. Limited value quotation is supported,
// but variable and command expansions are not supported.
func parseEnvFile(envInput io.Reader) (map[string]string, error) {
	envMap := make(map[string]string)

	scanner := bufio.NewScanner(envInput)
	var lineNumber int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNumber++

		// skip empty lines and lines starting with comment
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// split line into key and value
		before, after, isCut := strings.Cut(line, "=")
		if !isCut {
			return nil, fmt.Errorf("can't parse line %d; line should be in KEY=VALUE format", lineNumber)
		}
		key, val := before, after

		// sometimes keys are prefixed by "export " so file can be sourced in bash; ignore it here
		key = strings.TrimPrefix(key, "export ")

		// validate key and value
		if key == "" {
			return nil, fmt.Errorf("missing or empty key on line %d", lineNumber)
		}
		if strings.Contains(key, " ") {
			return nil, fmt.Errorf("invalid key on line %d: contains whitespace: %s", lineNumber, key)
		}
		if strings.HasPrefix(val, " ") || strings.HasPrefix(val, "\t") {
			return nil, fmt.Errorf("invalid value on line %d: whitespace before value: '%s'", lineNumber, val)
		}

		// remove any trailing comment after value
		if commentStart, _, found := strings.Cut(val, "#"); found {
			val = strings.TrimRight(commentStart, " \t")
		}

		// quoted value: support newlines
		if strings.HasPrefix(val, `"`) || strings.HasPrefix(val, "'") {
			quote := string(val[0])
			for !strings.HasSuffix(line, quote) || strings.HasSuffix(line, `\`+quote) {
				val = strings.ReplaceAll(val, `\`+quote, quote)
				if !scanner.Scan() {
					break
				}
				lineNumber++
				line = strings.ReplaceAll(scanner.Text(), `\`+quote, quote)
				val += "\n" + line
			}
			val = strings.TrimPrefix(val, quote)
			val = strings.TrimSuffix(val, quote)
		}

		envMap[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return envMap, nil
}
