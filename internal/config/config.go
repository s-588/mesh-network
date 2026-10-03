// package config parses environment variables or .env file into a Config struct
// for other packages to use.
package config

import (
	"flag"
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
		ID:            1234,
		TTL:           20,
		Lifetime:      30,
		HelloInterval: 5,
	},
	LogConfig: LogConfig{
		Level:   "INFO",
		LogFile: time.Now().Format(time.DateTime) + ".log",
	},
}

// Flag variables for command line arguments
var (
	flagIsDaemon = flag.Bool("daemon", false, "Start without GUI as daemon. You can't be abble to do anything, only view logs")

	flagPort          = flag.Uint("port", 0, "Port to listen on")
	flagInterface     = flag.String("interface", "", "One or multiple interfaces to listen on. Must be set with flag or env variable")
	flagID            = flag.Uint64("id", 0, "ID of this node. Must be set with flag or env variable")
	flagTTL           = flag.Uint("ttl", 0, "Time To Live for messages")
	flagLifetime      = flag.Uint("lifetime", 0, "Lifetime of messages and entrys in route table")
	flagHelloInterval = flag.Uint("hello_interval", 0, "Interval in which HELLO messages will be broadcasted")

	flagLogFile  = flag.String("log_file", "", "Log filename or full path")
	flagLogLevel = flag.String("log_level", "", "Level of logs that will be displayed")
)

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

// NewConfig parse environment variables, flags
// and creates new instances of Config
//
//nolint:funlen,gocyclo // function is long but it is only parsing and validating config
func NewConfig() (Config, error) {
	cfg := defaultConfig

	_ = godotenv.Load()

	flag.Parse()

	err := applyEnv(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("apply environment variables: %w", err)
	}

	if *flagIsDaemon {
		cfg.IsDaemon = *flagIsDaemon
	}

	if *flagLogFile != "" {
		cfg.LogFile = *flagLogFile
	}

	if *flagLogLevel != "" {
		cfg.Level = *flagLogLevel
	}

	if *flagPort != 0 {
		port := *flagPort
		if port < 1 || port > math.MaxUint16 {
			return cfg, fmt.Errorf("invalid port number: %d", port)
		}
		cfg.Port = uint16(port)
	}

	if *flagInterface != "" {
		ifacesStr := *flagInterface
		cfg.Interfaces = strings.Split(ifacesStr, ",")
	}

	if *flagID != 0 {
		cfg.ID = *flagID
	}

	if *flagTTL != 0 {
		ttl := *flagTTL
		if ttl < 1 || ttl > math.MaxUint8 {
			return cfg, fmt.Errorf("invalid TTL value: %d", ttl)
		}
		cfg.TTL = uint8(ttl)
	}

	if *flagLifetime != 0 {
		lifetime := *flagLifetime
		if lifetime < 1 || lifetime > math.MaxUint32 {
			return cfg, fmt.Errorf("invalid lifetime value: %d", lifetime)
		}
		cfg.Lifetime = uint32(lifetime)
	}

	if *flagHelloInterval != 0 {
		cfg.HelloInterval = int(*flagHelloInterval)
	}

	if cfg.ID == 0 {
		seed := rand.NewSource(time.Now().Unix())
		node, err := snowflake.NewNode(seed.Int63())
		if err != nil {
			return cfg, fmt.Errorf("creation snowflake id failed: %w", err)
		}
		id := node.Generate().Int64()
		slog.Warn("ID was not set, generating Snowflake ID", "id", id)
		if id < 0 {
			return cfg, fmt.Errorf("generated Snowflake ID is negative: %d", id)
		}
		cfg.ID = uint64(id)
	}

	return cfg, nil
}

// applyEnv set values from environment variables to cfg
//
//nolint:funlen,gocyclo // function is long but it is only applying envs to config
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

	sb.WriteString("\tLogger configuration:\n")
	fmt.Fprintf(&sb, "\t\tLevel: %s\n", cfg.Level)
	fmt.Fprintf(&sb, "\t\tLogFile: %s\n", cfg.LogFile)
	return sb.String()
}
