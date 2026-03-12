package settings

import "github.com/zalando/go-keyring"

const KeychainService = "com.circleci.cli"

func GetTokenFromKeychain(host string) (string, error) {
	return keyring.Get(KeychainService, host)
}

func SetTokenInKeychain(host, token string) error {
	return keyring.Set(KeychainService, host, token)
}

func DeleteTokenFromKeychain(host string) error {
	return keyring.Delete(KeychainService, host)
}
