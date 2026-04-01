package provider

import "arrai/internal/domain"

var (
	ProviderTypes = map[string]struct{}{
		"vk": {},
	}
)

type Provider interface {
	GetPosts(authorID string) (*[]domain.Post, error)
}
