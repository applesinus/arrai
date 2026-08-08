package vk

import (
	"arrai/internal/domain"
	"fmt"
	"strings"
	"time"
)

type Attachment struct {
	Type  *string `json:"type"`
	Photo struct {
		Sizes []struct {
			Url string `json:"url"`
		}
		OrigPhoto struct {
			Url *string `json:"url"`
		} `json:"orig_photo"`
	} `json:"photo"`
}

type vkWallPost struct {
	// ID
	ID *int `json:"id"`

	// OwnerID
	OwnerID *int `json:"owner_id"`

	// CreatedAt
	CreatedAt *int `json:"date"`

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
	Text *string `json:"text"`

	// Photos
	Attachments []Attachment `json:"attachments"`
}

func (p vkWallPost) toDomain() (domain.Post, error) {
	if p.ID == nil {
		return domain.Post{}, domain.ERROR_NO_POST_ID
	}
	if p.Text == nil {
		return domain.Post{}, domain.ERROR_NO_TEXT
	}

	if p.OwnerID == nil {
		p.OwnerID = p.OwnerID
	}

	creationTime := creationTimeOrDefault(p.CreatedAt)

	photos := make([]domain.PhotoWithPreview, 0)
	photos := make([]domain.Photo, 0)
	if p.Attachments != nil {
		for _, attachment := range p.Attachments {
			if attachment.Type == nil {
				continue
			}
			if attachment.Photo.OrigPhoto.Url == nil {
				if attachment.Photo.Sizes == nil || len(attachment.Photo.Sizes) == 0 {
					continue
				} else {
					attachment.Photo.OrigPhoto.Url = &attachment.Photo.Sizes[len(attachment.Photo.Sizes)-1].Url

			switch *attachment.Type {
			case ATTACHMENT_PHOTO:
				if attachment.Photo.OrigPhoto.Url == nil {
					if attachment.Photo.Sizes == nil || len(attachment.Photo.Sizes) == 0 {
						continue
					} else {
						attachment.Photo.OrigPhoto.Url = &attachment.Photo.Sizes[len(attachment.Photo.Sizes)-1].Url
					}
				}
			}

			if *attachment.Type == ATTACHMENT_PHOTO {
				smallSizeUrl := ""

				switch len(attachment.Photo.Sizes) {
				case 0:
					smallSizeUrl = *attachment.Photo.OrigPhoto.Url
				case 1:
					smallSizeUrl = attachment.Photo.Sizes[0].Url
				default:
					smallSizeUrl = attachment.Photo.Sizes[1].Url
				}

				photos = append(photos, domain.PhotoWithPreview{
					Self: domain.Photo{
				photos = append(photos, domain.Photo{
					Self: domain.Picture{
						Url: parseUrl(*attachment.Photo.OrigPhoto.Url),
					},
					Preview: domain.Photo{
					Preview: domain.Picture{
						Url: parseUrl(smallSizeUrl),
					},
				})
			}
		}
	}

	post := domain.Post{
		ID:        *p.ID,
		OwnerID:   *p.OwnerID,
		CreatedAt: creationTime,
		Views:     p.Views.Count,
		Reactions: p.Reactions.Count,
		Reposts:   p.Reposts.Count,
		Text:      *p.Text,
		Photos:    photos,
		Comments:  nil,
	}

	return post, nil
}

type vkComment struct {
	// Base
	ID *int `json:"id"`

	// CreatedAt
	CreatedAt *int `json:"date"`

	// Text
	User *int `json:"from_id"`

	Text  *string `json:"text"`
	Likes struct {
		Count *int `json:"count"`
	} `json:"likes"`

	Attachments []Attachment `json:"attachments"`
	Thread      struct {
		Count *int `json:"count"`
	} `json:"thread"`
}

func (c vkComment) toDomain(wallAuthorID int) (domain.Comment, error) {
	if c.ID == nil {
		return domain.Comment{}, domain.ERROR_NO_COMMENT_ID
	}

	creationTime := creationTimeOrDefault(c.CreatedAt)

	photos := make([]domain.PhotoWithPreview, 0)
	photos := make([]domain.Photo, 0)
	for _, attachment := range c.Attachments {
		if *attachment.Type == ATTACHMENT_PHOTO {
			smallSizeUrl := ""

			switch len(attachment.Photo.Sizes) {
			case 0:
				smallSizeUrl = *attachment.Photo.OrigPhoto.Url
			case 1:
				smallSizeUrl = attachment.Photo.Sizes[0].Url
			default:
				smallSizeUrl = attachment.Photo.Sizes[1].Url
			}

			photos = append(photos, domain.PhotoWithPreview{
				Self: domain.Photo{
			photos = append(photos, domain.Photo{
				Self: domain.Picture{
					Url: *attachment.Photo.OrigPhoto.Url,
				},
				Preview: domain.Photo{
				Preview: domain.Picture{
					Url: smallSizeUrl,
				},
			})
		}
	}

	return domain.Comment{
		ID:        *c.ID,
		CreatedAt: creationTime,

		User:      fmt.Sprintf("%d", c.User),
		IsAuthor:  *c.User == wallAuthorID,
		Reactions: *c.Likes.Count,

		Text:    *c.Text,
		Photos:  photos,
		Replies: nil,
	}, nil
}

// Support functions

func creationTimeOrDefault(creationTime *int) time.Time {
	returnInt := domain.PLACEHOLDER_TIME
	if creationTime != nil {
		returnInt = *creationTime
	}

	return time.Unix(int64(returnInt), 0).Truncate(0)
}

func parseUrl(url string) string {
	return strings.ReplaceAll(url, "\\u0026", "&")
}
