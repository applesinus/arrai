package provider

import "arrai/internal/domain"

type Provider interface {
	GetPosts(authorID string) (*[]domain.Post, error)
}
