package repository

import "arrai/internal/domain"

type WallRepository interface {
	SavePost(serviceName, authorID string, post domain.Post) (int, error)
	SavePosts(serviceName, authorID string, posts []domain.Post) (map[int]int, error)

	GetPost(serviceName, authorID string, postID int) (domain.Post, error)
	GetPosts(serviceName, authorID string, postIDs []int) ([]domain.Post, error)
	GetAllPosts(serviceName, authorID string) ([]domain.Post, error)
	GetExistingPostIDs(serviceName, authorID string) ([]int, error)

	UpdatePost(serviceName, authorID string, post domain.Post) error
	UpdatePosts(serviceName, authorID string, posts []domain.Post) error

	DeletePost(serviceName, authorID string, postID int) error
}
