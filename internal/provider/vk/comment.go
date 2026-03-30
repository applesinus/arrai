package vk

import (
	"fmt"

	"arrai/internal/domain"
)

type Comment struct {
	ID int

	User      string
	IsAuthor  bool
	Reactions int

	Text    string
	Photos  *[]string
	Replies *[]*Comment
}

func (c Comment) ToDomain() (domain.Comment, error) {
	domainReplies := make([]domain.Comment, 0)

	if c.Replies != nil && len(*c.Replies) > 0 {
		for _, reply := range *c.Replies {
			domainReply, err := reply.ToDomain()
			if err != nil {
				return domain.Comment{}, err
			}

			domainReplies = append(domainReplies, domainReply)
		}
	}

	return domain.Comment{
		ID:        c.ID,
		User:      c.User,
		IsAuthor:  c.IsAuthor,
		Reactions: c.Reactions,
		Text:      c.Text,
		// TODO Photos
		Photos:  make([]string, 0),
		Replies: domainReplies,
	}, nil
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

func (c vkComment) toComment() Comment {
	return Comment{
		ID: c.ID,

		User:      fmt.Sprintf("%d", c.User),
		IsAuthor:  c.IsAuthor,
		Reactions: c.Likes.Count,

		Text: c.Text,
		// TODO: Photos
		//Photos: c.Attachments,
		Replies: nil,
	}
}
