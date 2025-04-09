package config

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/inovacc/logger"
	"github.com/inovacc/utils/v2/uid"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	appName  = "app"
	instance *Config
	once     sync.Once
)

func init() {
	once.Do(func() {
		instance = &Config{
			fs:            afero.NewOsFs(),
			configPaths:   make([]string, 0),
			supportedExts: []string{"json", "yaml", "yml"},
			Logger: LoggerConfig{
				LogLevel:   slog.LevelDebug.String(),
				LogFormat:  "json",
				MaxSize:    100,
				MaxAge:     7,
				MaxBackups: 10,
				LocalTime:  true,
				Compress:   true,
			},
		}
	})

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

type ServiceConfig interface {
	DefaultValues() error
}

type LoggerConfig struct {
	LogLevel   string `yaml:"logLevel" mapstructure:"logLevel" json:"logLevel"`
	LogFormat  string `yaml:"logFormat" mapstructure:"logFormat" json:"logFormat"`
	FileName   string `yaml:"fileName" mapstructure:"fileName" json:"fileName"`
	MaxSize    int    `yaml:"maxSize" mapstructure:"maxSize" json:"maxSize"`
	MaxAge     int    `yaml:"maxAge" mapstructure:"maxAge" json:"maxAge"`
	MaxBackups int    `yaml:"maxBackups" mapstructure:"maxBackups" json:"maxBackups"`
	LocalTime  bool   `yaml:"localTime" mapstructure:"localTime" json:"localTime"`
	Compress   bool   `yaml:"compress" mapstructure:"compress" json:"compress"`
}

type Config struct {
	fs            afero.Fs
	configFile    string
	supportedExts []string
	configPaths   []string
	AppID         string        `yaml:"appID" mapstructure:"appID" json:"appID"`
	AppName       string        `yaml:"appName" mapstructure:"appName" json:"appName"`
	Logger        LoggerConfig  `yaml:"logger" mapstructure:"logger" json:"logger"`
	Service       ServiceConfig `yaml:"service" mapstructure:"service" json:"service"`
}

func (c *Config) defaultValues() error {
	if c.AppName == "" {
		c.AppName = appName
	}

	if c.Logger.FileName == "" {
		c.Logger.FileName = c.AppName
	}

	if c.AppID == "" {
		c.AppID = uid.GenerateKSUID()
	}

	opts := &slog.HandlerOptions{}

	switch c.Logger.LogLevel {
	case slog.LevelDebug.String():
		c.Logger.LogLevel = slog.LevelDebug.String()
		opts.Level = slog.LevelDebug
	case slog.LevelInfo.String():
		c.Logger.LogLevel = slog.LevelInfo.String()
		opts.Level = slog.LevelInfo
	case slog.LevelWarn.String():
		c.Logger.LogLevel = slog.LevelWarn.String()
		opts.Level = slog.LevelWarn
	case slog.LevelError.String():
		c.Logger.LogLevel = slog.LevelError.String()
		opts.Level = slog.LevelError
	default:
		return fmt.Errorf("unknown log level: %s", c.Logger.LogLevel)
	}

	if c.Logger.LogFormat == "" {
		c.Logger.LogFormat = "json"
	}

	logger.NewLoggerWithJSONRotator(logger.NewRotatorHandler(
		instance.Logger.FileName,
		instance.Logger.MaxSize,
		instance.Logger.MaxAge,
		instance.Logger.MaxBackups,
		instance.Logger.LocalTime,
		instance.Logger.Compress,
	), opts)

	return nil
}

func (c *Config) getConfigFile() (string, string, error) {
	if c.configFile == "" {
		cf, err := c.findConfigFile()
		if err != nil {
			return "", "", err
		}
		c.configFile = filepath.Clean(cf)
	}

	ext := strings.TrimPrefix(filepath.Ext(c.configFile), ".")
	if !contains(c.supportedExts, ext) {
		return "", "", fmt.Errorf("unsupported config file extension: %s", ext)
	}
	return c.configFile, ext, nil
}

// Find and return a valid configuration file.
func (c *Config) findConfigFile() (string, error) {
	slog.Info("Searching for configuration file", "paths", c.configPaths)
	for _, path := range c.configPaths {
		if file := c.searchInPath(path); file != "" {
			return file, nil
		}
	}
	return "", fmt.Errorf("no config file found in paths: %v", c.configPaths)
}

// Search for a config file in a specified path.
func (c *Config) searchInPath(path string) string {
	for _, ext := range c.supportedExts {
		filePath := filepath.Join(path, fmt.Sprintf("%s.%s", appName, ext))
		if exists(c.fs, filePath) {
			return filePath
		}
	}
	return ""
}

func (c *Config) readInConfig() error {
	slog.Info("attempting to read in config file")
	filename, ext, err := c.getConfigFile()
	if err != nil {
		return err
	}

	slog.Debug("reading file", "file", filename)
	file, err := afero.ReadFile(c.fs, filename)
	if err != nil {
		return err
	}

	viper.SetConfigType(ext)
	viper.SetConfigFile(filename)
	viper.AutomaticEnv()

	if err = viper.ReadConfig(bytes.NewReader(file)); err != nil {
		return fmt.Errorf("fatal error config file: %s", err)
	}

	if err = viper.Unmarshal(instance); err != nil {
		return fmt.Errorf("fatal error config file: %s", err)
	}

	viper.Reset() // clean viper instance

	return nil
}

// SetConfigService injects a user-defined config struct into the global
// config instance used by the service.
//
// This function allows merging user-provided configuration (which can be
// modified freely) with a fixed internal structure required for correct
// operation. It ensures mandatory fields are set by applying default
//
// Usage:
//
//	err := SetConfigService(MyCustomConfig{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// The resulting combined config struct containing both user-defined and required fields.
func SetConfigService(object ServiceConfig) error {
	instance.Service = object
	if err := instance.Service.DefaultValues(); err != nil {
		return err
	}
	return instance.defaultValues()
}

// GenerateDefaultConfig injects a user-defined config struct into the global
// config instance used by the service.
//
// This function allows merging user-provided configuration (which can be
// modified freely) with a fixed internal structure required for correct
// operation. It ensures mandatory fields are set by applying default
// values before writing the merged result to `config.yaml`.
//
// Usage:
//
//	err := GenerateDefaultConfig(MyCustomConfig{}, "/etc/myapp")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// The resulting `config.yaml` will contain both user-defined and required fields.
func GenerateDefaultConfig(object ServiceConfig) error {
	if err := SetConfigService(object); err != nil {
		return err
	}
	return writeToFile(filepath.Join("..", "..", "config.yaml"))
}

func GetConfig() *Config {
	return instance
}

func GetServiceConfig[T ServiceConfig]() T {
	return instance.Service.(T)
}

func InitConfig(object ServiceConfig) error {
	cfgFile := viper.GetString("config")

	cfgFileEnv := os.Getenv("CONFIG_FILE")
	if cfgFileEnv != "" {
		cfgFile = cfgFileEnv
	}

	if cfgFile == "" {
		return errors.New("no config file from params or CONFIG_FILE environment variable found")
	}

	configFile, err := filepath.Abs(cfgFile)
	if err != nil {
		return fmt.Errorf("invalid config file path: %w", err)
	}

	instance.configFile = configFile

	if err := SetConfigService(object); err != nil {
		return err
	}

	if err := instance.readInConfig(); err != nil {
		return fmt.Errorf("read in config: %s", err)
	}

	if err := instance.defaultValues(); err != nil {
		return fmt.Errorf("default values: %s", err)
	}

	return nil
}

func writeToFile(cfgFile string) error {
	file, err := os.Create(cfgFile)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	return encoder.Encode(instance)
}

// Check if a file exists.
func exists(fs afero.Fs, path string) bool {
	stat, err := fs.Stat(path)
	return err == nil && !stat.IsDir()
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
