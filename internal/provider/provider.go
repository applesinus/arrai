package provider

import (
	"arrai/internal/domain"
	"context"
)

var (
	ProviderTypes = map[string]struct{}{
		"vk": {},
	}
)

type Provider interface {
	GetPosts(ctx context.Context, authorID string) (*[]domain.Post, error)
}
