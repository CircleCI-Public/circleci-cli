package settings

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/data"
	yaml "gopkg.in/yaml.v3"
)

// Config is used to represent the current state of a CLI instance.
type Config struct {
	Host            string
	Endpoint        string
	Token           string
	RestEndpoint    string            `yaml:"rest_endpoint"`
	Data            *data.YML         `yaml:"-"`
	Debug           bool              `yaml:"-"`
	Address         string            `yaml:"-"`
	FileUsed        string            `yaml:"-"`
	GitHubAPI       string            `yaml:"-"`
	SkipUpdateCheck bool              `yaml:"-"`
	OrbPublishing   OrbPublishingInfo `yaml:"orb_publishing"`
}

type OrbPublishingInfo struct {
	DefaultNamespace   string `yaml:"default_namespace"`
	DefaultVcsProvider string `yaml:"default_vcs_provider"`
	DefaultOwner       string `yaml:"default_owner"`
}

// UpdateCheck is used to represent settings for checking for updates of the CLI.
type UpdateCheck struct {
	LastUpdateCheck time.Time `yaml:"last_update_check"`
	FileUsed        string    `yaml:"-"`
}

// Load will read the update check settings from the user's disk and then deserialize it into the current instance.
func (upd *UpdateCheck) Load() error {
	path := filepath.Join(SettingsPath(), updateCheckFilename())

	if err := ensureSettingsFileExists(path); err != nil {
		return err
	}

	upd.FileUsed = path

	content, err := ioutil.ReadFile(path) // #nosec
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(content, &upd)
	return err
}

// WriteToDisk will write the last update check to disk by serializing the YAML
func (upd *UpdateCheck) WriteToDisk() error {
	enc, err := yaml.Marshal(&upd)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(upd.FileUsed, enc, 0600)
	return err
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
	path := filepath.Join(SettingsPath(), configFilename())

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

	if restEndpoint := ReadFromEnv(prefix, "rest_endpoint"); restEndpoint != "" {
		cfg.RestEndpoint = restEndpoint
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

// updateCheckFilename returns the name of the cli update checks file
func updateCheckFilename() string {
	return "update_check.yml"
}

// configFilename returns the name of the cli config file
func configFilename() string {
	// TODO: Make this configurable
	return "cli.yml"
}

// settingsPath returns the path of the CLI settings directory
func SettingsPath() string {
	// TODO: Make this configurable
	home, _ := os.UserHomeDir()
	return path.Join(home, ".circleci")
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
