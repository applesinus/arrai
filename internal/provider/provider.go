package provider

import "arrai/internal/domain"

var (
	ProviderTypes = []string{"vk"}
)

type Provider interface {
	GetPosts(authorID string) (*[]domain.Post, error)
}
