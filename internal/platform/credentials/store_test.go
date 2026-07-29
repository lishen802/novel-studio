package credentials

import (
	"context"
	"errors"
	"testing"
)

func TestStoreDelegatesSecretsWithoutRetainingThem(t *testing.T) {
	backend := &memoryBackend{secrets: map[string]string{}}
	store := newStore(backend)

	if err := store.Save(context.Background(), "provider:one", "top-secret"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	secret, err := store.Get(context.Background(), "provider:one")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if secret != "top-secret" {
		t.Fatalf("secret = %q", secret)
	}
	if err := store.Delete(context.Background(), "provider:one"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(context.Background(), "provider:one"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("expected ErrCredentialNotFound, got %v", err)
	}
}

func TestStoreRejectsBlankCredentialValues(t *testing.T) {
	store := newStore(&memoryBackend{secrets: map[string]string{}})
	if err := store.Save(context.Background(), "", "secret"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
	if err := store.Save(context.Background(), "provider:one", " "); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
}

type memoryBackend struct {
	secrets map[string]string
}

func (backend *memoryBackend) save(_ context.Context, reference, secret string) error {
	backend.secrets[reference] = secret
	return nil
}

func (backend *memoryBackend) get(_ context.Context, reference string) (string, error) {
	secret, exists := backend.secrets[reference]
	if !exists {
		return "", ErrCredentialNotFound
	}
	return secret, nil
}

func (backend *memoryBackend) delete(_ context.Context, reference string) error {
	delete(backend.secrets, reference)
	return nil
}
