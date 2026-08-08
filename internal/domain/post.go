package domain

import (
	"errors"
	"time"
)

var (
	ERROR_NO_POST_ID    = errors.New("no post id")
	ERROR_NO_TEXT       = errors.New("no text")
	ERROR_NO_COMMENT_ID = errors.New("no comment id")

	PLACEHOLDER_TIME = int(time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC).Unix())
)

type Post struct {
	ID        int       `json:"id"`
	OwnerID   int       `json:"ownerId"`
	CreatedAt time.Time `json:"createdAt"`
	Views     int       `json:"views"`
	Reactions int       `json:"reactions"`
	Reposts   int       `json:"reposts"`
	Text      string    `json:"text"`

	Photos []Photo `json:"photos"`

	Comments []Comment `json:"comments"`
}

type Comment struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	User      string    `json:"user"`
	IsAuthor  bool      `json:"isAuthor"`
	Reactions int       `json:"reactions"`
	Text      string    `json:"text"`
	Photos    []Photo   `json:"photos"`
	Replies   []Comment `json:"replies"`
}

type Photo struct {
	Self    Picture `json:"photo"`
	Preview Picture `json:"preview"`
}

type Picture struct {
	Url      string `json:"url"`
	Filename string `json:"filename"`

	Content []byte `json:"content"`
}

}
