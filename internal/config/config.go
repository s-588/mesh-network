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
			Port:   6040,
			Ifaces: []string{"eth0"},
			ID:     1234,
		},
		LogConfig: LogConfig{
			Level:   "INFO",
			LogFile: time.Now().Format(time.DateTime) + ".log",
		},
	}
)

type Config struct {
	LogConfig
	AppConfig
}

type AppConfig struct {
	ID     uint64
	Port   uint16
	Ifaces []string
}

type LogConfig struct {
	Level   string
	LogFile string
}

func NewConfig() (Config, error) {
	cfg := defaultConfig
	node, err := snowflake.NewNode(rand.Int63n(1024))
	if err != nil {
		return cfg, fmt.Errorf("creation snowflake id failed: %w", err)
	}
	cfg.ID = uint64(node.Generate().Int64())

	if err := godotenv.Load(); err != nil {
		slog.Warn("can't load environment variables", "error", err)
	}

	flagPort := flag.Uint("port", 0, "Port to listen on")
	flagInterface := flag.String("interfaces", "", "One or multiple interfaces to listen on. Must be set with flag or env variable")
	flagID := flag.Uint64("id", 0, "ID of this node. Must be set with flag or env variable")

	flagLogFile := flag.String("log", "", "Log filename or full path")
	flagLogLevel := flag.String("log_level", "", "Level of logs that will be displayed")
	flag.Parse()

	err = applyEnv(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("apply environment variables: %w", err)
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

	if *flagLogFile != "" {
		cfg.LogFile = string(*flagLogFile)
	}

	if *flagLogLevel != "" {
		cfg.Level = string(*flagLogLevel)
	}

	return cfg, nil
}

func applyEnv(cfg *Config) error {
	// Port
	if s := os.Getenv("PORT"); s != "" {
		v, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			return fmt.Errorf("parse PORT: %w", err)
		}
		cfg.Port = uint16(v)
	}

	// Interface
	if s := os.Getenv("INTERFACE"); s != "" {
		cfg.AppConfig.Ifaces = strings.Split(strings.ReplaceAll(s, " ", ""), ",")
	}

	// ID
	if s := os.Getenv("ID"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parse ID: %w", err)
		}
		cfg.ID = v
	}

	// Log file
	if s := os.Getenv("LOG_FILE"); s != "" {
		cfg.LogFile = path.Clean(s)
	} else {
		dir, _ := os.UserHomeDir()
		cfg.LogFile = path.Join(dir, "mesh-network", defaultConfig.LogFile)
	}

	// Log level
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
