package vk

import (
	"arrai/internal/domain"
)

type Post struct {
	ID int

	Views     int
	Reactions int
	Reposts   int

	Text     string
	Photos   *[]string
	Comments *[]*Comment
}

func (p Post) ToDomain() (domain.Post, error) {
	domainComments := make([]domain.Comment, 0)

	if p.Comments != nil && len(*p.Comments) > 0 {
		for _, comment := range *p.Comments {
			domainComment, err := comment.ToDomain()
			if err != nil {
				return domain.Post{}, err
			}

			domainComments = append(domainComments, domainComment)
		}
	}

	return domain.Post{
		ID:        p.ID,
		Views:     p.Views,
		Reactions: p.Reactions,
		Reposts:   p.Reposts,
		Text:      p.Text,
		// TODO: Photos
		Photos:   make([]string, 0),
		Comments: domainComments,
	}, nil
}

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

func (p vkWallPost) toPost() (Post, error) {
	post := Post{
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

func (p Post) ToJSON() ([]byte, error) {
	return nil, nil
}
