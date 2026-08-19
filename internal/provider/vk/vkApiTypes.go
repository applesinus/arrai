package vk

import (
	"arrai/internal/domain"
	"fmt"
	"log/slog"
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
	Video struct {
		Id          int    `json:"id"`
		OwnerId     int    `json:"owner_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Image       []struct {
			Height int    `json:"height"`
			Url    string `json:"url"`
		} `json:"image"`
	}
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

func (p vkWallPost) toDomain(logger slog.Logger) (domain.Post, error) {
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
	photos, videos := parseAttachments(logger, p.Attachments, *p.ID)

	post := domain.Post{
		ID:        *p.ID,
		OwnerID:   *p.OwnerID,
		CreatedAt: creationTime,
		Views:     p.Views.Count,
		Reactions: p.Reactions.Count,
		Reposts:   p.Reposts.Count,
		Text:      *p.Text,
		Photos:    photos,
		Videos:    videos,
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

func (c vkComment) toDomain(logger slog.Logger, wallAuthorID int) (domain.Comment, error) {
	if c.ID == nil {
		return domain.Comment{}, domain.ERROR_NO_COMMENT_ID
	}

	creationTime := creationTimeOrDefault(c.CreatedAt)
	photos, videos := parseAttachments(logger, c.Attachments, *c.ID)

	return domain.Comment{
		ID:        *c.ID,
		CreatedAt: creationTime,

		User:      fmt.Sprintf("%d", c.User),
		IsAuthor:  *c.User == wallAuthorID,
		Reactions: *c.Likes.Count,

		Text:    *c.Text,
		Photos:  photos,
		Videos:  videos,
		Replies: nil,
	}, nil
}

// SUPPORT FUNCTIONS

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

func parseAttachments(logger slog.Logger, attachments []Attachment, parentID int) ([]domain.Photo, []domain.Video) {
	photos := make([]domain.Photo, 0)
	videos := make([]domain.Video, 0)

	if attachments != nil && len(attachments) != 0 {
		for _, attachment := range attachments {
			if attachment.Type == nil {
				logger.Error("No attachment type",
					"parent_id", parentID,
					"attachment", attachment,
				)
				continue
			}

			switch *attachment.Type {
			case ATTACHMENT_PHOTO:
				if attachment.Photo.OrigPhoto.Url == nil {
					if attachment.Photo.Sizes == nil || len(attachment.Photo.Sizes) == 0 {
						continue
					} else {
						attachment.Photo.OrigPhoto.Url = &attachment.Photo.Sizes[len(attachment.Photo.Sizes)-1].Url
					}
				}

				smallSizeUrl := ""

				switch len(attachment.Photo.Sizes) {
				case 0:
					smallSizeUrl = *attachment.Photo.OrigPhoto.Url
				case 1:
					smallSizeUrl = attachment.Photo.Sizes[0].Url
				default:
					smallSizeUrl = attachment.Photo.Sizes[1].Url
				}

				photos = append(photos, domain.Photo{
					Original: domain.Picture{
						Url: parseUrl(*attachment.Photo.OrigPhoto.Url),
					},
					Preview: domain.Picture{
						Url: parseUrl(smallSizeUrl),
					},
				})

			case ATTACHMENT_VIDEO:
				imgMaxHeight := 0
				previewUrl := ""
				for _, img := range attachment.Video.Image {
					if img.Height > imgMaxHeight {
						imgMaxHeight = img.Height
						previewUrl = img.Url
					}
				}

				videos = append(videos, domain.Video{
					Url:         fmt.Sprintf("https://vk.com/clip%d_%d", attachment.Video.OwnerId, attachment.Video.Id),
					Title:       attachment.Video.Title,
					Description: attachment.Video.Description,
					Preview: domain.Picture{
						Url: parseUrl(previewUrl),
					},
				})

			default:
				logger.Error("Unsupported attachment type",
					"parent_id", parentID,
					"attachment", attachment,
				)
			}
		}
	}

	return photos, videos
}
