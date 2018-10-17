package settings

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
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

// Load will read the config from the user's disk and then evaluate possible configuration from the environment.
func (cfg *Config) Load() error {
	if err := cfg.LoadFromDisk(); err != nil {
		return err
	}

	cfg.LoadFromEnv("circleci_cli")

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
