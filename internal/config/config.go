// package config parses environment variables or .env file into a Config struct
// for other packages to use.
//
// CLI flags are handled exclusively by urfave/cli in main — this package only
// provides defaults, environment-variable overrides, and optional snowflake ID
// generation (only when EnsureID is called, i.e. when actually starting a node).
package config

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/snowflake"
	"github.com/joho/godotenv"
)

var defaultConfig = Config{
	AppConfig: AppConfig{
		Port:          6040,
		Interfaces:    []string{"eth0"},
		ID:            0, // 0 means "not set" → EnsureID will generate a snowflake
		TTL:           20,
		Lifetime:      30,
		HelloInterval: 5,
	},
	LogConfig: LogConfig{
		Level:   "INFO",
		LogFile: time.Now().Format(time.DateTime) + ".log",
	},
}

// Config struct contain all variables under the users control
type Config struct {
	IsDaemon bool
	LogConfig
	AppConfig
}

// AppConfig struct contain variables that belongs to routing or protocol logic
type AppConfig struct {
	ID            uint64
	Port          uint16
	Interfaces    []string // interfaces on which app is listening
	TTL           uint8
	Lifetime      uint32 // lifetime for RREP and route table in seconds
	HelloInterval int
}

// LogConfig struct contain variables for logger
type LogConfig struct {
	Level   string
	LogFile string
}

// NewConfig loads defaults, optional .env file, and environment variables.
// Call EnsureID() only when actually starting a node (daemon).
//
//nolint:funlen // parsing only
func NewConfig() (Config, error) {
	cfg := defaultConfig

	_ = godotenv.Load()

	if err := applyEnv(&cfg); err != nil {
		return cfg, fmt.Errorf("apply environment variables: %w", err)
	}

	return cfg, nil
}

// EnsureID generates a snowflake node ID when cfg.ID is still 0.
// Call this only from the daemon startup path , otherwise every invocation logs a
// WARN and invents a throwaway ID.
func (cfg *Config) EnsureID() error {
	if cfg.ID != 0 {
		return nil
	}
	seed := rand.NewSource(time.Now().UnixNano())
	node, err := snowflake.NewNode(seed.Int63() % 1024) // valid range 0–1023
	if err != nil {
		return fmt.Errorf("creation snowflake id failed: %w", err)
	}
	id := node.Generate().Int64()
	slog.Warn("ID was not set, generating Snowflake ID", "id", id)
	if id < 0 {
		return fmt.Errorf("generated Snowflake ID is negative: %d", id)
	}
	cfg.ID = uint64(id)
	return nil
}

// applyEnv set values from environment variables to cfg
//
//nolint:funlen,gocyclo // function is long but it just applies values
func applyEnv(cfg *Config) error {
	if s := os.Getenv("PORT"); s != "" {
		v, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			return fmt.Errorf("parse PORT: %w", err)
		}
		cfg.Port = uint16(v)
	}

	if s := os.Getenv("INTERFACE"); s != "" {
		if ifaces := strings.Split(strings.ReplaceAll(s, " ", ""), ","); len(ifaces) != 0 {
			cfg.Interfaces = ifaces
		} else {
			return fmt.Errorf("parsed INTERFACE is empty")
		}
	}

	if s := os.Getenv("ID"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parse ID: %w", err)
		}
		cfg.ID = v
	}

	if s := os.Getenv("TTL"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parse TTL: %w", err)
		}
		if v < 1 || v > math.MaxUint8 {
			return fmt.Errorf("invalid TTL value: %d", v)
		}
		cfg.TTL = uint8(v)
	}

	if s := os.Getenv("LIFETIME"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parse LIFETIME: %w", err)
		}
		if v < 1 || v > math.MaxUint32 {
			return fmt.Errorf("invalid lifetime value: %d", v)
		}
		cfg.Lifetime = uint32(v)
	}

	if s := os.Getenv("HELLO_INTERVAL"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parse HELLO_INTERVAL: %w", err)
		}
		if v < 1 || v > math.MaxInt {
			return fmt.Errorf("invalid HELLO_INTERVAL value: %d", v)
		}
		cfg.HelloInterval = int(v)
	}

	if s := os.Getenv("LOG_FILE"); s != "" {
		cfg.LogFile = path.Clean(s)
	} else {
		dir, err := os.UserHomeDir()
		if err != nil {
			cfg.LogFile = "mesh-network_" + defaultConfig.LogFile
		} else {
			cfg.LogFile = path.Join(dir, defaultConfig.LogFile)
		}
	}

	if s := os.Getenv("LOG_LEVEL"); s != "" {
		s = strings.Map(func(r rune) rune {
			if !unicode.IsLetter(r) {
				return -1
			}
			return r
		}, s)
		s = strings.TrimSpace(s)
		s = strings.ToUpper(s)
		cfg.Level = s
	} else {
		cfg.Level = defaultConfig.Level
	}

	if s := os.Getenv("DAEMON"); s != "" {
		v, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("parse DAEMON: %w", err)
		}
		cfg.IsDaemon = v
	}

	return nil
}

func (cfg *Config) String() string {
	sb := strings.Builder{}
	sb.WriteString("Configuration:\n")
	fmt.Fprintf(&sb, "\tIsDaemon: %t\n", cfg.IsDaemon)

	sb.WriteString("\tApplication configuration:\n")
	fmt.Fprintf(&sb, "\t\tID: %d\n", cfg.ID)
	fmt.Fprintf(&sb, "\t\tPort: %d\n", cfg.Port)
	sb.WriteString("\t\tInterfaces: ")
	for i, iface := range cfg.Interfaces {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(iface)
	}
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "\t\tTTL: %d\n", cfg.TTL)
	fmt.Fprintf(&sb, "\t\tLifetime: %d seconds\n", cfg.Lifetime)
	fmt.Fprintf(&sb, "\t\tHelloInterval: %d seconds\n", cfg.HelloInterval)

	sb.WriteString("\tLogger configuration:\n")
	fmt.Fprintf(&sb, "\t\tLevel: %s\n", cfg.Level)
	fmt.Fprintf(&sb, "\t\tLogFile: %s\n", cfg.LogFile)
	return sb.String()
}
