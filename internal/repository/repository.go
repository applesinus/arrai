package repository

import "arrai/internal/domain"

type WallRepository interface {
	SavePost(post domain.Post) (int, error)
	SavePosts(posts []domain.Post) (map[int]int, error)

	GetPost(postID int) (domain.Post, error)
	GetPosts(postIDs []int) ([]domain.Post, error)
	GetAllPosts() ([]domain.Post, error)
	GetExistingPostIDs() ([]int, error)

	UpdatePost(post domain.Post) error
	UpdatePosts(posts []domain.Post) error

	DeletePost(postID int) error
}
