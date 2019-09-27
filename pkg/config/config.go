package config

import (
	"github.com/spf13/viper"
)

// Config represents all the config for go_mirror
type Config struct {
	ConfigFile     string `yaml:"file" mapstructure:"file" toml:"file"`
	UpstreamMirror string `yaml:"mirror" mapstructure:"mirror" toml:"mirror"`
	PrimaryServer  string `yaml:"server" mapstructure:"server" toml:"server"`
}

// Option configures the configuration struct
type Option func(opt *Config) error

// WithConfigFile will configure the Config struct using
// the configured config file
var WithConfigFile = func(configFile string) Option {
	return func(cfg *Config) error {
		cfg.ConfigFile = configFile
		return nil
	}
}

// InitConfig returns a Config struct,
// configured by the opts provided
func InitConfig(opts ...Option) (*Config, error) {
	viper.AutomaticEnv()

	cfg := new(Config)
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	if cfg.ConfigFile != "" {
		viper.SetConfigFile(cfg.ConfigFile)
		if err := viper.ReadInConfig(); err != nil {
			return cfg, err
		}
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
