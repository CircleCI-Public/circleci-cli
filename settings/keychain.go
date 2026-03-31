package settings

import (
	"errors"
	"time"

	"github.com/zalando/go-keyring"
)

const KeychainService = "com.circleci.cli"

// keychainTimeout is how long we wait for a keychain call before giving up.
// On macOS CI runners the Security framework can block indefinitely waiting
// for user authorisation; treating that as "unavailable" is the right fallback.
const keychainTimeout = 3 * time.Second

var errKeychainTimeout = errors.New("keychain operation timed out")

func keychainDo(fn func() error) error {
	ch := make(chan error, 1)
	go func() { ch <- fn() }()
	select {
	case err := <-ch:
		return err
	case <-time.After(keychainTimeout):
		return errKeychainTimeout
	}
}

func GetTokenFromKeychain(host string) (string, error) {
	var token string
	err := keychainDo(func() error {
		var e error
		token, e = keyring.Get(KeychainService, host)
		return e
	})
	return token, err
}

func SetTokenInKeychain(host, token string) error {
	return keychainDo(func() error {
		return keyring.Set(KeychainService, host, token)
	})
}

func DeleteTokenFromKeychain(host string) error {
	return keychainDo(func() error {
		return keyring.Delete(KeychainService, host)
	})
}
