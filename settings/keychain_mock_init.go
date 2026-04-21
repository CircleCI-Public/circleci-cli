//go:build keychain_mock

package settings

import "github.com/zalando/go-keyring"

// init calls MockInit when the binary is compiled with -tags=keychain_mock
// (i.e. in integration test builds) so keychain calls use an in-memory store
// instead of the OS keychain, preventing hangs on CI runners.
func init() {
	keyring.MockInit()
}
