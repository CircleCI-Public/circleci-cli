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

// EnsureSettingsFileExists does just that.
func EnsureSettingsFileExists(filepath, filename string) error {
	// TODO - handle invalid YAML config files.
	_, err := os.Stat(filepath)

	if !os.IsNotExist(err) {
		return nil
	}

	if err = os.MkdirAll(filepath, 0700); err != nil {
		return err
	}

	if _, err = os.Create(path.Join(filepath, filename)); err != nil {
		return err
	}

	return nil
}
