package vk

import (
	"context"
	"log/slog"
)

type vkError struct {
	Code    int    `json:"error_code"`
	Message string `json:"error_msg"`
}

func (e vkError) IsNotNull(ctx context.Context, logger *slog.Logger) bool {
	if e.Code != 0 || e.Message != "" {
		logger.Debug("Error detected",
			"error_code", e.Code,
			"error_message", e.Message,
		)
		return true
	}

	return false
}

type vkGetWallResponse struct {
	Response struct {
		Items []vkWallPost `json:"items"`
	} `json:"response"`
	Error vkError `json:"error"`
}

type vkResolveNameResponse struct {
	Response struct {
		ID int `json:"object_id"`
	} `json:"response"`
	Error vkError `json:"error"`
}

type vkGetCommentsResponse struct {
	Response struct {
		Items []vkComment `json:"items"`
	} `json:"response"`
	Error vkError `json:"error"`
}

type vkGetPostStatsResponse struct {
	Response []vkPostStats `json:"response"`
	Error    vkError       `json:"error"`
}
