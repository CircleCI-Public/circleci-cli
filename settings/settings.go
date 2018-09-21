package settings

import (
	"os"
	"path"
	"runtime"
)

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
func ConfigFilename() string {
	// TODO: Make this configurable
	return "cli.yml"
}

// ConfigPath returns the path of the cli config file
func ConfigPath() string {
	// TODO: Make this configurable
	return path.Join(UserHomeDir(), ".circleci")
}

// EnsureSettingsFileExists does just that.
func EnsureSettingsFileExists(filepath, filename string) error {
	// TODO - handle invalid YAML config files.

	fullPath := path.Join(filepath, filename)

	_, err := os.Stat(fullPath)

	if err == nil {
		return nil
	}

	if !os.IsNotExist(err) {
		// Filesystem error
		return err
	}

	// Create folder
	if err = os.MkdirAll(filepath, 0700); err != nil {
		return err
	}

	_, err = os.Create(fullPath)
	return err
}
