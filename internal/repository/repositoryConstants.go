package repository

import (
	"errors"
)

var (
	ERR_POST_NOT_FOUND = errors.New("post not found")
	ERR_POST_EXISTS    = errors.New("post already exists")
)
