package mirror

import (
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

type Header struct {
	Key   string
	Value string
}

type MirrorConfig struct {
	URL     string
	Headers []Header
}

type DockerLookupConfig struct {
	HostIdentifier string `yaml:"host-identifier" toml:"host-identifier" mapstructure:"host-identifier"`
	Enabled        bool
}

type PrimaryConfig struct {
	URL             string
	Headers         []Header
	DoMirrorHeaders bool `yaml:"do-mirror-headers" toml:"do-mirror-headers" mapstructure:"do-mirror-headers"`
	DoMirrorBody    bool `yaml:"do-mirror-body" toml:"do-mirror-body" mapstructure:"do-mirror-body"`
	// Lookup the domain in docker based on HostIdentifier
	DockerLookup DockerLookupConfig `yaml:"docker-lookup-config" toml:"docker-lookup-config" mapstructure:"docker-lookup-config"`
}

// Config represents all the config for gomirror
type Config struct {
	ConfigFile string `yaml:"file" toml:"file" mapstructure:"file"`
	Port       int
	Mirror     MirrorConfig
	Primary    PrimaryConfig
	LogLevel   string `yaml:"log-level" toml:"log-level" mapstructure:"log-level"`
	LogFile    string `yaml:"log-file" toml:"log-file" mapstructure:"log-file"`
	viper      *viper.Viper
}

func parsedHTTPHeaders(headers []Header) http.Header {
	httpHeaders := http.Header(make(map[string][]string))

	for _, header := range headers {
		httpHeaders[header.Key] = []string{header.Value}
	}

	return httpHeaders
}

// HTTPHeaders parses headers into a valid http.Header
func (c *MirrorConfig) HTTPHeaders() http.Header {
	return parsedHTTPHeaders(c.Headers)
}

// HTTPHeaders parses headers into a valid http.Header
func (c *PrimaryConfig) HTTPHeaders() http.Header {
	return parsedHTTPHeaders(c.Headers)
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

// WithViper uses a custom viper instance
var WithViper = func(v *viper.Viper) Option {
	return func(cfg *Config) error {
		cfg.viper = v
		return nil
	}
}

// InitConfig returns a Config struct,
// configured by the opts provided
func InitConfig(opts ...Option) (*Config, error) {
	cfg := new(Config)
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	if cfg.viper == nil {
		cfg.viper = viper.GetViper()
	}

	viper := cfg.viper
	if cfg.ConfigFile != "" {
		viper.SetConfigFile(cfg.ConfigFile)
		if err := viper.ReadInConfig(); err != nil {
			return cfg, err
		}
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return cfg, err
	}

	if viper.ConfigFileUsed() == "" {
		//try and parse headers
		for _, kvPair := range viper.GetStringSlice("primary-headers") {
			pair := strings.SplitN(kvPair, "=", 2)
			if len(pair) < 2 {
				continue
			}
			key := pair[0]
			value := pair[1]
			cfg.Primary.Headers = append(cfg.Primary.Headers, Header{
				Key:   key,
				Value: value,
			})
		}

		for _, kvPair := range viper.GetStringSlice("mirror-headers") {
			pair := strings.SplitN(kvPair, "=", 2)
			if len(pair) < 2 {
				continue
			}
			key := pair[0]
			value := pair[1]
			cfg.Mirror.Headers = append(cfg.Mirror.Headers, Header{
				Key:   key,
				Value: value,
			})
		}
	}

	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	}

	if cfg.Port == 0 {
		cfg.Port = 80
	}

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			logrus.WithError(err).
				Errorln("error opening log file")
		}
		logrus.SetOutput(f)
		logrus.WithField("file", cfg.LogFile).Infoln("using file output")
	}

	return cfg, nil
}
