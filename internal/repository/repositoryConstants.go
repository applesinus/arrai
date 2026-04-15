package repository

import (
	"errors"
)

const (
	BASE_PATH_ENV_KEY = "DISK_REPO_BASE_PATH"

	PHOTO_SMALL_PREFIX = "small_"
)

var (
	ERR_POST_NOT_FOUND  = errors.New("post not found")
	ERR_POST_EXISTS     = errors.New("post already exists")
	ERR_PHOTO_EXISTS    = errors.New("photo already exists")
	ERR_PHOTO_NOT_FOUND = errors.New("photo not found")
)
