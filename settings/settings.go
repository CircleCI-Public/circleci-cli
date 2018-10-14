package settings

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// Config is used to represent the current state of a CLI instance.
// It's passed around internally among commands to persist an HTTP client, Logger and other useful data.
type Config struct {
	Client   *client.Client `yaml:"-"`
	Logger   *logger.Logger `yaml:"-"`
	Host     string
	Endpoint string
	Token    string
	Debug    bool   `yaml:"-"`
	Address  string `yaml:"-"`
	FileUsed string `yaml:"-"`
}

// NewLogger will initialize a new logger instance for a CLI config to be shared.
// We're assuming debug has already been parsed before calling this function or will be initialized with the default "false" boolean value.
func (cfg *Config) NewLogger() *logger.Logger {
	return logger.NewLogger(cfg.Debug)
}

// NewClient initializes a new HTTP client to be shared inside the CLI
// The address, token, and logger must have already been set prior to calling this function.
func (cfg *Config) NewClient() *client.Client {
	return client.NewClient(cfg.Address, cfg.Token, cfg.Logger)
}

// GraphQLServerAddress returns the full address to CircleCI GraphQL API server
func GraphQLServerAddress(endpoint, host string) (string, error) {
	// 1. Parse the endpoint
	e, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.Wrapf(err, "Parsing endpoint '%s'", endpoint)
	}

	// 2. Parse the host
	h, err := url.Parse(host)
	if err != nil {
		return "", errors.Wrapf(err, "Parsing host '%s'", host)
	}
	if !h.IsAbs() {
		return h.String(), fmt.Errorf("Host (%s) must be absolute URL, including scheme", host)
	}

	// 3. Resolve the two URLs using host as the base
	// We use ResolveReference which has specific behavior we can rely for
	// older configurations which included the absolute path for the endpoint flag.
	//
	// https://golang.org/pkg/net/url/#URL.ResolveReference
	//
	// Specifically this function always returns the reference (endpoint) if provided an absolute URL.
	// This way we can safely introduce --host and merge the two.
	return h.ResolveReference(e).String(), err
}

// Load will read the config from the user's disk and then evaluate possible configuration from the environment.
func (cfg *Config) Load() error {
	if err := cfg.LoadFromDisk(); err != nil {
		return err
	}

	cfg.LoadFromEnv("circleci_cli")

	return nil
}

// Setup will initialize a logger and http client for the CLI
func (cfg *Config) Setup() error {
	cfg.Logger = cfg.NewLogger()
	address, err := GraphQLServerAddress(cfg.Endpoint, cfg.Host)
	if err != nil {
		return err
	}
	cfg.Address = address
	cfg.Client = cfg.NewClient()

	return nil
}

// LoadFromDisk is used to read config from the user's disk and deserialize the YAML into our runtime config.
func (cfg *Config) LoadFromDisk() error {
	path := filepath.Join(configPath(), configFilename())

	if err := ensureSettingsFileExists(path); err != nil {
		return err
	}

	cfg.FileUsed = path

	content, err := ioutil.ReadFile(path) // #nosec
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(content, &cfg)
	return err
}

// WriteToDisk will write the runtime config instance to disk by serializing the YAML
func (cfg *Config) WriteToDisk() error {
	enc, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(cfg.FileUsed, enc, 0600)
	return err
}

// LoadFromEnv will read from environment variables of the given prefix for host, endpoint, and token specifically.
func (cfg *Config) LoadFromEnv(prefix string) {
	if host := ReadFromEnv(prefix, "host"); host != "" {
		cfg.Host = host
	}

	if endpoint := ReadFromEnv(prefix, "endpoint"); endpoint != "" {
		cfg.Endpoint = endpoint
	}

	if token := ReadFromEnv(prefix, "token"); token != "" {
		cfg.Token = token
	}
}

// ReadFromEnv takes a prefix and field to search the environment for after capitalizing and joining them with an underscore.
func ReadFromEnv(prefix, field string) string {
	name := strings.Join([]string{prefix, field}, "_")
	return os.Getenv(strings.ToUpper(name))
}

// UserHomeDir returns the path to the current user's HOME directory.
func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

// ConfigFilename returns the name of the cli config file
func configFilename() string {
	// TODO: Make this configurable
	return "cli.yml"
}

// ConfigPath returns the path of the cli config file
func configPath() string {
	// TODO: Make this configurable
	return path.Join(UserHomeDir(), ".circleci")
}

// ensureSettingsFileExists does just that.
func ensureSettingsFileExists(path string) error {
	// TODO - handle invalid YAML config files.

	_, err := os.Stat(path)

	if err == nil {
		return nil
	}

	if !os.IsNotExist(err) {
		// Filesystem error
		return err
	}

	dir := filepath.Dir(path)

	// Create folder
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	_, err = os.Create(path)
	if err != nil {
		return err
	}

	err = os.Chmod(path, 0600)

	return err
}
