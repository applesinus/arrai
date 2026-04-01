package domain

import "time"

type Post struct {
	ID        int       `json:"id"`
	OwnerID   int       `json:"ownerId"`
	CreatedAt time.Time `json:"createdAt"`
	Views     int       `json:"views"`
	Reactions int       `json:"reactions"`
	Reposts   int       `json:"reposts"`
	Text      string    `json:"text"`
	Photos    []string  `json:"photos"`
	Comments  []Comment `json:"comments"`
}

type Comment struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	User      string    `json:"user"`
	IsAuthor  bool      `json:"isAuthor"`
	Reactions int       `json:"reactions"`
	Text      string    `json:"text"`
	Photos    []string  `json:"photos"`
	Replies   []Comment `json:"replies"`
}
