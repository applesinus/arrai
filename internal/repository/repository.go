package repository

import (
	"arrai/internal/domain"
	"context"
)

type Repository interface {
	SavePost(ctx context.Context, post domain.Post) (int, error)
	SavePosts(ctx context.Context, posts []domain.Post) (map[int]int, error)

	GetPost(ctx context.Context, postID int) (domain.Post, error)
	GetPosts(ctx context.Context, postIDs []int) ([]domain.Post, error)
	GetAllPosts(ctx context.Context) ([]domain.Post, error)
	GetExistingPostIDs(ctx context.Context) ([]int, error)

	UpdatePost(ctx context.Context, post domain.Post) error
	UpdatePosts(ctx context.Context, posts []domain.Post) error

	DeletePost(ctx context.Context, postID int) error
	Clear(ctx context.Context) error
}
