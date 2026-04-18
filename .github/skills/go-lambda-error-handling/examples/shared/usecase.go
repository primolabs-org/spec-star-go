package shared

import (
	"context"
	"fmt"
)

type Repository interface {
	Load(ctx context.Context, id string) (string, error)
}

type UseCase struct {
	repo Repository
}

func NewUseCase(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", ErrInvalidInput
	}

	value, err := uc.repo.Load(ctx, id)
	if err != nil {
		return "", fmt.Errorf("load entity %q: %w", id, err)
	}

	return value, nil
}
