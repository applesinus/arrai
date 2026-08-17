package repository

import (
	"arrai/internal/domain"
	"context"
)

type Repository interface {
	// === SAVE ===

	// SavePost saves one post
	SavePost(ctx context.Context, post *domain.Post) error

	// SavePosts saves multiple posts to the disk repo
	//
	// Ranges over the slice and try to save each post. Returning a combined error in the end of the range
	//
	// Not returning an error if there is no posts in the slice
	SavePosts(ctx context.Context, posts *map[int]domain.Post) error

	// === GET ===

	// GetPost returns one post or an error if something failed
	GetPost(ctx context.Context, postID int) (domain.Post, error)

	// GetPosts returns multiple posts or an error if something failed
	//
	// Ranges over the slice and try to get each post. Returning a combined error in the end of the range
	GetPosts(ctx context.Context, postIDs []int) (map[int]domain.Post, error)

	// GetExistingPostIDs returns all existing post ids
	GetExistingPostIDs(ctx context.Context) ([]int, error)

	// GetAllPosts returns all posts
	//
	// Ranges over the database and try to get every post. Returning a combined error in the end of the range
	GetAllPosts(ctx context.Context) (map[int]domain.Post, error)

	// === UPDATE ===

	// UpdatePost updates one post
	UpdatePost(ctx context.Context, post domain.Post) error

	// UpdatePosts updates multiple posts
	//
	// Ranges over the slice and try to update each post. Returning a combined error in the end of the range
	UpdatePosts(ctx context.Context, posts map[int]domain.Post) error

	// UpdateOrSavePost updates a post if it exists, otherwise saves it
	UpdateOrSavePost(ctx context.Context, post *domain.Post) error

	// UpdateOrSavePosts updates a post if it exists, otherwise saves it
	//
	// Ranges over the slice and try to update or save each post. Returning a combined error in the end of the range
	UpdateOrSavePosts(posts *map[int]domain.Post) error

	// === DELETE ===

	// DeletePost deletes one post
	DeletePost(ctx context.Context, postID int) error

	// DeletePosts deletes multiple posts
	//
	// Ranges over the slice and try to delete each post. Returning a combined error in the end of the range
	DeletePosts(ctx context.Context, postIDs []int) error

	// Clear clears entire repo
	Clear(ctx context.Context) error
}
