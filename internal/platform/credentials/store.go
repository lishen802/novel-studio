package credentials

import (
	"context"
	"errors"
	"strings"
)

const serviceName = "NovelStudio"

var (
	ErrCredentialNotFound = errors.New("credential not found")
	ErrInvalidCredential  = errors.New("invalid credential")
	ErrCredentialStore    = errors.New("credential store operation failed")
)

type backend interface {
	save(ctx context.Context, reference, secret string) error
	get(ctx context.Context, reference string) (string, error)
	delete(ctx context.Context, reference string) error
}

type Store struct {
	backend backend
}

func NewStore() *Store {
	return newStore(newPlatformBackend())
}

func newStore(backend backend) *Store {
	return &Store{backend: backend}
}

func (store *Store) Save(ctx context.Context, reference, secret string) error {
	reference = strings.TrimSpace(reference)
	secret = strings.TrimSpace(secret)
	if reference == "" || secret == "" {
		return ErrInvalidCredential
	}
	return store.backend.save(ctx, reference, secret)
}

func (store *Store) Get(ctx context.Context, reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", ErrInvalidCredential
	}
	return store.backend.get(ctx, reference)
}

func (store *Store) Delete(ctx context.Context, reference string) error {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return ErrInvalidCredential
	}
	return store.backend.delete(ctx, reference)
}
