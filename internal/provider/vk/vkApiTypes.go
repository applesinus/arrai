package vk

import (
	"arrai/internal/domain"
	"fmt"
)

type vkWallPost struct {
	// Views
	Views struct {
		Count int `json:"count"`
	} `json:"views"`

	// Reactions
	Reactions struct {
		Count int `json:"count"`
	} `json:"reactions"`

	// Reposts
	Reposts struct {
		Count int `json:"count"`
	} `json:"reposts"`

	// Text
	Text string `json:"text"`

	// Photos
	Attachments []struct {
		Type  string `json:"type"`
		Photo struct {
			OrigPhoto struct {
				Url string `json:"url"`
			} `json:"orig_photo"`
		} `json:"photo"`
	} `json:"attachments"`

	// ID is for comments
	ID int `json:"id"`
}

func (p vkWallPost) toDomain() (domain.Post, error) {
	creationTime := time.Unix(int64(p.CreatedAt), 0)

	post := domain.Post{
		ID:        p.ID,
		Views:     p.Views.Count,
		Reactions: p.Reactions.Count,
		Reposts:   p.Reposts.Count,
		Text:      p.Text,
		Photos:    nil,
		Comments:  nil,
	}

	return post, nil
}

type vkComment struct {
	// Base
	ID int `json:"id"`

	// Text
	User     int  `json:"from_id"`
	IsAuthor bool `json:"is_from_post_author"`

	Text  string `json:"text"`
	Likes struct {
		Count int `json:"count"`
	} `json:"likes"`

	Attachments []struct {
		Type  string `json:"type"`
		Photo struct {
			OrigPhoto struct {
				Url string `json:"url"`
			} `json:"orig_photo"`
		} `json:"photo"`
	} `json:"attachments"`
	Thread struct {
		Count int `json:"count"`
	} `json:"thread"`
}

func (c vkComment) toDomain() domain.Comment {
	creationTime := time.Unix(int64(c.CreatedAt), 0)

	return domain.Comment{
		ID:        c.ID,

		User:      fmt.Sprintf("%d", c.User),
		IsAuthor:  c.IsAuthor,
		Reactions: c.Likes.Count,

		Text: c.Text,
		// TODO: Photos
		//Photos: c.Attachments,
		Replies: nil,
	}
}
