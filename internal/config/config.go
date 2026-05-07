package config

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	LogConfig
	AppConfig
}

type AppConfig struct {
	Ifaces []string
}

type LogConfig struct {
	Level string
	File  string
}

func NewConfig() (Config, error) {
	err := godotenv.Load()
	if err != nil {
		return Config{}, fmt.Errorf("can't load envs: %w", err)
	}

	appCfg, err := parseAppConfig()

	return Config{
		LogConfig: parseLogConfig(),
		AppConfig: appCfg,
	}, nil
}

func parseLogConfig() LogConfig {
	return LogConfig{
		Level: parseLogLevel(),
		File:  parseLogFile(),
	}
}

func parseLogFile() string {
	filename := time.Now().Format(time.DateTime) + ".log"
	s := os.Getenv("LOG_FILE")
	if s == "" {
		dir, _ := os.UserHomeDir()
		return path.Join(dir, "mesh-network", filename)
	}
	return filename
}

func parseLogLevel() string {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = slog.LevelInfo.String()
	}
	return level
}

func parseAppConfig() (AppConfig,error) {
	ifaces,err := parseIfaces()
	if err != nil {
		return AppConfig{}, err
	}
	return AppConfig{
		Ifaces: ifaces,
	},nil
}

func parseIfaces()([]string,error){
	ifaces := make([]string,0)
	ifaceStr := os.Getenv("INTERFACE")
	if ifaceStr == ""{
		return nil, fmt.Errorf("Interface is not specified")
	}
	ifaces = strings.Split(strings.ReplaceAll(ifaceStr," ",""),",")
	return ifaces, nil
}
