package config

import (
	"flag"
	"fmt"
	"log/slog"
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

var (
	defaultConfig = Config{
		AppConfig: AppConfig{
			Port:     6040,
			Ifaces:   []string{"eth0"},
			ID:       1234,
			TTL:      20,
			Lifetime: 30,
		},
		LogConfig: LogConfig{
			Level:   "INFO",
			LogFile: time.Now().Format(time.DateTime) + ".log",
		},
	}
)

// Config struct contain all user variables under the users control
type Config struct {
	IsDaemon bool
	LogConfig
	AppConfig
}

// AppConfig struct contain variables that belongs to routing or protocol logic
type AppConfig struct {
	ID       uint64
	Port     uint16
	Ifaces   []string
	TTL      uint8
	Lifetime uint32 // lifetime for RREP and route table in seconds
}

// LogConfig struct contain variables for logger
type LogConfig struct {
	Level   string
	LogFile string
}

// NewConfig parse environment variables, flags
// and creates new instances of Config
func NewConfig() (Config, error) {
	cfg := defaultConfig

	_ = godotenv.Load()

	flagIsDaemon := flag.Bool("daemon", false, "Start without GUI as daemon. You can't be abble to do anything, only view logs")

	flagPort := flag.Uint("port", 0, "Port to listen on")
	flagInterface := flag.String("interface", "", "One or multiple interfaces to listen on. Must be set with flag or env variable")
	flagID := flag.Uint64("id", 0, "ID of this node. Must be set with flag or env variable")
	flagTTL := flag.Uint("ttl", 0, "Time To Live for messages")
	flagLifetime := flag.Uint("lifetime", 0, "Lifetime of messages and entrys in route table")

	flagLogFile := flag.String("log_file", "", "Log filename or full path")
	flagLogLevel := flag.String("log_level", "", "Level of logs that will be displayed")
	flag.Parse()

	err := applyEnv(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("apply environment variables: %w", err)
	}

	if *flagIsDaemon != false {
		cfg.IsDaemon = bool(*flagIsDaemon)
	}

	if *flagPort != 0 {
		cfg.Port = uint16(*flagPort)
	}

	if *flagInterface != "" {
		ifacesStr := string(*flagInterface)
		cfg.Ifaces = strings.Split(ifacesStr, ",")
	}

	if *flagID != 0 {
		cfg.ID = uint64(*flagID)
	}

	if *flagTTL != 0 {
		cfg.TTL = uint8(*flagTTL)
	}

	if *flagLifetime != 0 {
		cfg.Lifetime = uint32(*flagLifetime)
	}

	if *flagLogFile != "" {
		cfg.LogFile = string(*flagLogFile)
	}

	if *flagLogLevel != "" {
		cfg.Level = string(*flagLogLevel)
	}

	if cfg.ID == 0 {
		node, err := snowflake.NewNode(rand.Int63n(1024))
		if err != nil {
			return cfg, fmt.Errorf("creation snowflake id failed: %w", err)
		}
		id := node.Generate().Int64()
		slog.Warn("ID was not set, generating Snowflake ID", "id", id)
		cfg.ID = uint64(id)
	}

	return cfg, nil
}

// applyEnv set values from environment variables to cfg
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
			cfg.Ifaces = ifaces
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
		cfg.TTL = uint8(v)
	}

	if s := os.Getenv("LIFETIME"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parse LIFETIME: %w", err)
		}
		cfg.Lifetime = uint32(v)
	}

	if s := os.Getenv("LOG_FILE"); s != "" {
		cfg.LogFile = path.Clean(s)
	} else {
		dir, _ := os.UserHomeDir()
		cfg.LogFile = path.Join(dir, "mesh-network", defaultConfig.LogFile)
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
	sb.WriteString(fmt.Sprintf("\tIsDaemon: %t\n", cfg.IsDaemon))

	sb.WriteString("\tApplication configuration:\n")
	sb.WriteString(fmt.Sprintf("\t\tID: %d\n", cfg.ID))
	sb.WriteString(fmt.Sprintf("\t\tPort: %d\n", cfg.Port))
	sb.WriteString("\t\tInterfaces: ")
	for i, iface := range cfg.Ifaces {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(iface)
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("\t\tTTL: %d\n", cfg.TTL))
	sb.WriteString(fmt.Sprintf("\t\tLifetime: %d seconds\n", cfg.Lifetime))

	sb.WriteString("\tLogger configuration:\n")
	sb.WriteString(fmt.Sprintf("\t\tLevel: %s\n", cfg.Level))
	sb.WriteString(fmt.Sprintf("\t\tLogFile: %s\n", cfg.LogFile))
	return sb.String()
}
