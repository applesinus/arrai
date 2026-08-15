package repository

import (
	"errors"
)

const (
	BASE_PATH_ENV_KEY = "DISK_REPO_BASE_PATH"

	PREVIEW_PREFIX = "preview_"
)

var (
	ERR_EMPTY_BASE_PATH = errors.New("empty base path")
	ERR_EMPTY_PROVIDER  = errors.New("empty provider")
	ERR_EMPTY_AUTHOR    = errors.New("empty author")
	ERR_NO_LOGGER       = errors.New("no logger")
	ERR_NO_APP_ENV      = errors.New("no app env")

	ERR_POST_NOT_FOUND  = errors.New("post not found")
	ERR_POST_EXISTS     = errors.New("post already exists")
	ERR_PHOTO_EXISTS    = errors.New("photo already exists")
	ERR_PHOTO_NOT_FOUND = errors.New("photo not found")
)
