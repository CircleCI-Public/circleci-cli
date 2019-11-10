package settings

import (
	"github.com/zalando/go-keyring"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/data"
	"gopkg.in/yaml.v2"
)

// Config is used to represent the current state of a CLI instance.
type Config struct {
	Host            string
	Endpoint        string
	Token           string
	Data            *data.YML `yaml:"-"`
	Debug           bool      `yaml:"-"`
	Address         string    `yaml:"-"`
	FileUsed        string    `yaml:"-"`
	GitHubAPI       string    `yaml:"-"`
	SkipUpdateCheck bool      `yaml:"-"`
}

// configYAML concludes all configuration values that will be written into a YAML file.
type configYAML struct {
	Host            string
	Endpoint        string
	Data            *data.YML `yaml:"-"`
	Debug           bool      `yaml:"-"`
	Address         string    `yaml:"-"`
	FileUsed        string    `yaml:"-"`
	GitHubAPI       string    `yaml:"-"`
	SkipUpdateCheck bool      `yaml:"-"`
}

// configKeyring concludes all configuration values that will be stored in the system keyring.
type configKeyring struct {
	Token string
}

// UpdateCheck is used to represent settings for checking for updates of the CLI.
type UpdateCheck struct {
	LastUpdateCheck time.Time `yaml:"last_update_check"`
	FileUsed        string    `yaml:"-"`
}

// Load will read the update check settings from the user's disk and then deserialize it into the current instance.
func (upd *UpdateCheck) Load() error {
	path := filepath.Join(settingsPath(), updateCheckFilename())

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
	path := filepath.Join(settingsPath(), configFilename())

	if err := ensureSettingsFileExists(path); err != nil {
		return err
	}

	cfg.FileUsed = path

	content, err := ioutil.ReadFile(path) // #nosec
	if err != nil {
		return err
	}

	var cfgYAML configYAML
	err = yaml.Unmarshal(content, &cfgYAML)

	var cfgKeyring configKeyring
	if token, err := keyring.Get("circleci", "circleci_cli"); err == nil {
		cfgKeyring.Token = token
	}

	cfg.merge(&cfgYAML, &cfgKeyring)

	return err
}

// WriteToDisk will write the runtime config instance to disk by serializing the YAML
func (cfg *Config) WriteToDisk() error {
	cfgYAML, cfgKeyring := cfg.split()

	enc, err := yaml.Marshal(&cfgYAML)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(cfg.FileUsed, enc, 0600); err != nil {
		return err
	}

	if err = keyring.Set("circleci", "circleci_cli", cfgKeyring.Token); err != nil {
		return err
	}

	return nil
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

// split splits the configuration into configuration types which will be stored in different ways.
// For example, some values will be written into a YAML file while other ones will be stored in the system keyring.
func (cfg *Config) split() (configYAML, configKeyring) {
	cfgYAML := configYAML{
		Host:            cfg.Host,
		Endpoint:        cfg.Endpoint,
		Data:            cfg.Data,
		Debug:           cfg.Debug,
		Address:         cfg.Address,
		FileUsed:        cfg.FileUsed,
		GitHubAPI:       cfg.GitHubAPI,
		SkipUpdateCheck: cfg.SkipUpdateCheck,
	}

	cfgKeyring := configKeyring{
		Token: cfg.Token,
	}

	return cfgYAML, cfgKeyring
}

// merge merges the given configuration types into the Config instance. This function is the reverse operation of split.
func (cfg *Config) merge(cfgYAML *configYAML, cfgKeyring *configKeyring) {
	cfg.Host = cfgYAML.Host
	cfg.Endpoint = cfgYAML.Endpoint
	cfg.Token = cfgKeyring.Token
	cfg.Data = cfgYAML.Data
	cfg.Debug = cfgYAML.Debug
	cfg.Address = cfgYAML.Address
	cfg.FileUsed = cfgYAML.FileUsed
	cfg.GitHubAPI = cfgYAML.GitHubAPI
	cfg.SkipUpdateCheck = cfgYAML.SkipUpdateCheck
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
func settingsPath() string {
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
