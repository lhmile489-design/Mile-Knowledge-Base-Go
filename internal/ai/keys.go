package ai

import (
	"errors"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// MaxKeyBytes matches mygo.MaxSecretSize (secrets.go:58).
const MaxKeyBytes = 2560

// maxKeyNameLen matches the settings-page validator (appConfig.ts:87).
const maxKeyNameLen = 96

// KeyBackend is the storage seam. Tests use memory; production uses the OS
// secret store; the real-store test uses a throwaway service name.
type KeyBackend interface {
	Set(name, value string) error
	Get(name string) (string, error)
	Delete(name string) error
}

// KeyStore validates names and values before handing them to a backend.
type KeyStore struct {
	backend KeyBackend
}

// NewKeyStore wraps a backend.
func NewKeyStore(backend KeyBackend) *KeyStore { return &KeyStore{backend: backend} }

// Set stores one key. Empty values are refused: removing a key is Delete.
func (s *KeyStore) Set(envName, value string) *contract.Failure {
	if !envNamePattern.MatchString(envName) || len(envName) > maxKeyNameLen {
		return contract.Fail(contract.CodeInvalidInput, "key 名称非法")
	}
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return contract.Fail(contract.CodeInvalidInput, "key 格式不正确")
	}
	if len(value) > MaxKeyBytes {
		return contract.Fail(contract.CodeTooLarge, "key 超过 2560 字节")
	}
	if err := s.backend.Set(envName, value); err != nil {
		return backendFailure(err)
	}
	return nil
}

// Get returns the plaintext key; only the engine should call it.
func (s *KeyStore) Get(envName string) (string, *contract.Failure) {
	value, err := s.backend.Get(envName)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return "", contract.Fail(contract.CodeNotFound, "还没有配置 API key")
		}
		return "", backendFailure(err)
	}
	return value, nil
}

// Has reports whether a key is configured, without revealing it.
func (s *KeyStore) Has(envName string) bool {
	if envName == "" {
		return false
	}
	_, err := s.backend.Get(envName)
	return err == nil
}

// Delete removes a key; deleting a missing key succeeds.
func (s *KeyStore) Delete(envName string) *contract.Failure {
	if err := s.backend.Delete(envName); err != nil && !errors.Is(err, errNotFound) {
		return backendFailure(err)
	}
	return nil
}

var errNotFound = errors.New("secret not found")

func backendFailure(err error) *contract.Failure {
	if errors.Is(err, errors.ErrUnsupported) {
		return contract.Fail(contract.CodeSecretUnavailable, "系统加密存储不可用，无法安全保存 API key")
	}
	if errors.Is(err, errNotFound) {
		return contract.Fail(contract.CodeNotFound, "还没有配置 API key")
	}
	return contract.Fail(contract.CodeIOFailure, "凭据存储失败")
}

// MemoryKeyStore is a KeyStore over an in-memory backend (unit tests, e2e).
func NewMemoryKeyStore() *KeyStore { return NewKeyStore(&memoryBackend{values: map[string]string{}}) }

type memoryBackend struct {
	mu     sync.Mutex
	values map[string]string
}

func (m *memoryBackend) Set(name, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[name] = value
	return nil
}

func (m *memoryBackend) Get(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[name]
	if !ok {
		return "", errNotFound
	}
	return value, nil
}

func (m *memoryBackend) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.values[name]; !ok {
		return errNotFound
	}
	delete(m.values, name)
	return nil
}

// SecretsBackend stores keys in the OS secret store through mygo.Secrets. The
// default service is the application's identifier; tests pass a throwaway
// name so they never touch the app's real credentials.
type SecretsBackend struct {
	module mygo.SecretsModule
}

// NewSecretsBackend opens the secret store for one service name. An empty
// service means the application's own identifier.
func NewSecretsBackend(service string) SecretsBackend {
	if service == "" {
		return SecretsBackend{module: mygo.Secrets}
	}
	return SecretsBackend{module: mygo.Secrets.Service(service)}
}

func (s SecretsBackend) Set(name, value string) error {
	return s.module.Set(name, []byte(value))
}

func (s SecretsBackend) Get(name string) (string, error) {
	value, err := s.module.Get(name)
	if err != nil {
		return "", normalizeSecretError(err)
	}
	return string(value), nil
}

func (s SecretsBackend) Delete(name string) error {
	err := s.module.Delete(name)
	if err != nil {
		return normalizeSecretError(err)
	}
	return nil
}

func normalizeSecretError(err error) error {
	if errors.Is(err, errors.ErrUnsupported) {
		return err
	}
	// mygo reports a missing secret as "secret not found"; keep the sentinel
	// machine-checkable for callers.
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return errNotFound
	}
	return err
}
