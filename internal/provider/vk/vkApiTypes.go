package vk

import (
	"arrai/internal/domain"
	"fmt"
	"time"
)

type Attachment struct {
	Type  string `json:"type"`
	Photo struct {
		Sizes []struct {
			Url string `json:"url"`
		}
		OrigPhoto struct {
			Url string `json:"url"`
		} `json:"orig_photo"`
	} `json:"photo"`
}

type vkWallPost struct {
	// OwnerID
	OwnerID int `json:"owner_id"`

	// CreatedAt
	CreatedAt int `json:"date"`

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
	Attachments []Attachment `json:"attachments"`

	// ID is for comments
	ID int `json:"id"`
}

func (p vkWallPost) toDomain() (domain.Post, error) {
	creationTime := time.Unix(int64(p.CreatedAt), 0)

	photos := make([]domain.TwoSizesPhoto, 0)
	for _, attachment := range p.Attachments {
		if attachment.Type == ATTACHMENT_PHOTO {
			smallSizeUrl := ""

			switch len(attachment.Photo.Sizes) {
			case 0:
				smallSizeUrl = attachment.Photo.OrigPhoto.Url
			case 1:
				smallSizeUrl = attachment.Photo.Sizes[0].Url
			default:
				smallSizeUrl = attachment.Photo.Sizes[1].Url
			}

			photos = append(photos, domain.TwoSizesPhoto{
				BigSize: domain.Photo{
					Url: attachment.Photo.OrigPhoto.Url,
				},
				SmallSize: domain.Photo{
					Url: smallSizeUrl,
				},
			})
		}
	}

	post := domain.Post{
		ID:        p.ID,
		OwnerID:   p.OwnerID,
		CreatedAt: creationTime,
		Views:     p.Views.Count,
		Reactions: p.Reactions.Count,
		Reposts:   p.Reposts.Count,
		Text:      p.Text,
		Photos:    photos,
		Comments:  nil,
	}

	return post, nil
}

type vkComment struct {
	// Base
	ID int `json:"id"`

	// CreatedAt
	CreatedAt int `json:"date"`

	// Text
	User     int  `json:"from_id"`
	IsAuthor bool `json:"is_from_post_author"`

	Text  string `json:"text"`
	Likes struct {
		Count int `json:"count"`
	} `json:"likes"`

	Attachments []Attachment `json:"attachments"`
	Thread      struct {
		Count int `json:"count"`
	} `json:"thread"`
}

func (c vkComment) toDomain() domain.Comment {
	creationTime := time.Unix(int64(c.CreatedAt), 0)

	photos := make([]domain.TwoSizesPhoto, 0)
	for _, attachment := range c.Attachments {
		if attachment.Type == ATTACHMENT_PHOTO {
			smallSizeUrl := ""

			switch len(attachment.Photo.Sizes) {
			case 0:
				smallSizeUrl = attachment.Photo.OrigPhoto.Url
			case 1:
				smallSizeUrl = attachment.Photo.Sizes[0].Url
			default:
				smallSizeUrl = attachment.Photo.Sizes[1].Url
			}

			photos = append(photos, domain.TwoSizesPhoto{
				BigSize: domain.Photo{
					Url: attachment.Photo.OrigPhoto.Url,
				},
				SmallSize: domain.Photo{
					Url: smallSizeUrl,
				},
			})
		}
	}

	return domain.Comment{
		ID:        c.ID,
		CreatedAt: creationTime,

		User:      fmt.Sprintf("%d", c.User),
		IsAuthor:  c.IsAuthor,
		Reactions: c.Likes.Count,

		Text:    c.Text,
		Photos:  photos,
		Replies: nil,
	}
}
