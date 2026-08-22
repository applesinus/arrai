package vk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"arrai/config/appEnv"
	"arrai/internal/domain"
	"arrai/internal/provider"
)

type Client struct {
	wg     *sync.WaitGroup
	logger *slog.Logger
	env    *appEnv.AppEnv

	accessToken string
	limiter     *rate.Limiter
	httpClient  *http.Client

	debugMode         bool
	maxPostsInRequest int
}

func NewClient(wg *sync.WaitGroup, logger *slog.Logger, appEnv *appEnv.AppEnv, accessToken string) provider.Provider {
	client := &Client{
		wg:     wg,
		logger: logger,
		env:    appEnv,

		accessToken: accessToken,
		limiter:     rate.NewLimiter(rate.Limit(appEnv.GetIntOrDefault("VK_MAX_RPS", 1)), 1),
		httpClient:  http.DefaultClient,

		debugMode:         appEnv.GetBoolOrDefault("DEBUG_MODE", false),
		maxPostsInRequest: appEnv.GetIntOrDefault("VK_MAX_POSTS_IN_REQUEST", 1),
	}

	return provider.Provider(client)
}

func (c *Client) createVkApiLink(method string, params map[string]any) string {
	paramsBuilder := strings.Builder{}
	for key, value := range params {
		paramsBuilder.WriteString(fmt.Sprintf("&%s=%v", key, value))
	}
	paramsString := paramsBuilder.String()

	c.logger.Debug("Link built",
		"link_without_token", fmt.Sprintf("%s%s?access_token=%s%s&v=%s", BASE_URL, method, "TokenRemoved", paramsString, VK_API_VERSION),
	)

	return fmt.Sprintf("%s%s?access_token=%s%s&v=%s", BASE_URL, method, c.accessToken, paramsString, VK_API_VERSION)
}

func (c *Client) doVkApiRequest(ctx context.Context, method string, params map[string]any) ([]byte, error) {
	err := c.limiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Get(c.createVkApiLink(method, params))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) getAuthorIntID(ctx context.Context, authorID string) (int, error) {
	resp, err := c.doVkApiRequest(ctx, METHOD_RESOLVE_NAME, map[string]any{"screen_name": authorID})
	if err != nil {
		return 0, err
	}

	response := vkResolveNameResponse{}
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return 0, err
	}
	if response.Error.IsNotNull(ctx, c.logger) {
		return 0, errors.New(response.Error.Message)
	}
	if response.Response.ID == 0 {
		return 0, fmt.Errorf("failed to resolve author ID: %s", authorID)
	}

	return response.Response.ID, nil
}

func (c *Client) GetPosts(ctx context.Context, authorID string) (*[]domain.Post, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	authorId, err := c.getAuthorIntID(ctx, authorID)
	if err != nil {
		return nil, err
	}
	authorId *= -1

	// Getting all posts
	posts := make([]domain.Post, 0)

	for offset := 0; ; offset += c.maxPostsInRequest {
		resp, err := c.doVkApiRequest(ctx, METHOD_GET_WALL, map[string]any{
			"domain": authorID,
			"count":  c.maxPostsInRequest,
			"offset": offset,
		})
		if err != nil {
			return nil, err
		}

		response := vkGetWallResponse{}
		err = json.Unmarshal(resp, &response)
		if err != nil {
			return nil, err
		}
		if response.Error.IsNotNull(ctx, c.logger) {
			return nil, errors.New(response.Error.Message)
		}

		if len(response.Response.Items) == 0 {
			break
		}

		for _, post := range response.Response.Items {
			newPost, err := post.toDomain(*c.logger)
			if err != nil {
				return nil, err
			}
			posts = append(posts, newPost)

			if c.debugMode {
				break
			}
		}

		if c.debugMode {
			break
		}

		c.logger.Info("Got posts batch",
			"iteration", offset/100+1,
			"total", len(posts),
		)
	}

	// Enriching posts
	for i := range posts {
		// Getting comments
		comments, errComments := c.getComments(ctx, authorId, posts[i].ID)
		if errComments != nil {
			c.logger.Error("failed to get comments for post",
				"post_id", posts[i].ID,
				"error", errComments,
			)
			if errComments == context.Canceled {
				break
			}
			continue
		}
		posts[i].Comments = *comments

		if comments != nil {
			c.logger.Debug("Got post's comments",
				"post_id", posts[i].ID,
				"total", len(*comments),
				"has_error", errComments != nil,
			)
		}

		// Getting photos
		errPhotos := c.fillPhotos(ctx, &posts[i].Photos)
		if errPhotos != nil {
			if errPhotos == context.Canceled {
				break
			}
			c.logger.Error("failed to get photos for post",
				"post_id", posts[i].ID,
				"error", errPhotos,
			)
			continue
		}

		c.logger.Debug("Got post's photos",
			"post_id", posts[i].ID,
			"total", len(posts[i].Photos),
			"has_error", errPhotos != nil,
		)

		// Getting videos
		errVideos := c.fillVideos(ctx, &posts[i].Videos)
		if errVideos != nil {
			if errVideos == context.Canceled {
				break
			}
			c.logger.Error("failed to get videos for post",
				"post_id", posts[i].ID,
				"error", errVideos,
			)
			continue
		}

		c.logger.Debug("Got post's videos",
			"post_id", posts[i].ID,
			"total", len(posts[i].Videos),
			"has_error", errVideos != nil,
		)

		// Getting stats
		errStats := c.fillStats(ctx, &posts[i])
		if errStats != nil {
			if errStats == context.Canceled {
				break
			}
			c.logger.Error("failed to get stats for post",
				"post_id", posts[i].ID,
				"error", errStats,
				"hint", fmt.Sprintf("consider checking method constraints on https://vk.com/dev/%s", METHOD_GET_POST_STATS),
			)
			continue
		}

		c.logger.Debug("Got post's stats",
			"post_id", posts[i].ID,
			"has_error", errStats != nil,
		)

		if c.debugMode {
			break
		}
	}

	return &posts, nil
}

func (c *Client) fillPhotos(ctx context.Context, photos *[]domain.Photo) error {
	for i := range *photos {
		photo, err := c.downloadPhoto(ctx, (*photos)[i].Original.Url)
		if err != nil {
			return err
		}

		preview := photo
		previewUrl := (*photos)[i].Preview.Url

		if previewUrl != (*photos)[i].Original.Url {
			preview, err = c.downloadPhoto(ctx, previewUrl)
			if err != nil {
				c.logger.Error("Cannot download small size photo, using big size copy instead",
					"error", err,
					"photo_url", previewUrl,
				)

				preview = photo
				previewUrl = (*photos)[i].Original.Url
			}
		}

		(*photos)[i].Original.Content = photo
		(*photos)[i].Preview.Content = preview
		(*photos)[i].Preview.Url = previewUrl
	}

	return nil
}

func (c *Client) downloadPhoto(ctx context.Context, url string) ([]byte, error) {
	c.limiter.Wait(ctx)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) fillVideos(ctx context.Context, videos *[]domain.Video) error {
	for i := range *videos {
		video, err := c.downloadVideo(ctx, (*videos)[i].Url)
		if err != nil {
			return err
		}
		if video == nil || len(video) == 0 {
			continue
		}

		(*videos)[i].Content = video

		preview, err := c.downloadPhoto(ctx, (*videos)[i].Preview.Url)
		if err != nil {
			preview, err = c.getFitstFrameFromVideo(ctx, video)
			if err != nil {
				return err
			}
			if len(preview) == 0 {
				preview = nil
			}
		}

		(*videos)[i].Preview.Content = preview
	}

	return nil
}

func (c *Client) downloadVideo(ctx context.Context, url string) ([]byte, error) {
	c.limiter.Wait(ctx)

	cmd := exec.CommandContext(ctx, "./third_party/bin/yt-dlp",
		"-o", "-",
		"--quiet",
		"--no-warnings",
		url,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %v (stderr: %s)", err, stderr.String())
	}

	return data, nil
}

func (c *Client) getFitstFrameFromVideo(ctx context.Context, video []byte) ([]byte, error) {
	// TODO
	return nil, nil
}

func (c *Client) getComments(ctx context.Context, authorID, postID int) (*[]domain.Comment, error) {
	comments := make([]domain.Comment, 0)

	for offset := 0; ; offset += 10 {
		resp, err := c.doVkApiRequest(ctx, METHOD_GET_COMMENTS, map[string]any{
			"owner_id":   authorID,
			"post_id":    postID,
			"need_likes": 1,

			"count":  10,
			"offset": offset,
		})
		if err != nil {
			return nil, err
		}

		response := vkGetCommentsResponse{}
		err = json.Unmarshal(resp, &response)
		if err != nil {
			return nil, err
		}
		if response.Error.IsNotNull(ctx, c.logger) {
			return nil, errors.New(response.Error.Message)
		}
		if len(response.Response.Items) == 0 {
			break
		}

		for _, comment := range response.Response.Items {
			newComment, err := comment.toDomain(*c.logger, authorID)
			if err != nil {
				return nil, err
			}
			comments = append(comments, newComment)

			if len(comments[len(comments)-1].Photos) > 0 {
				err = c.fillPhotos(ctx, &comments[len(comments)-1].Photos)
				if err != nil {
					c.logger.Error("failed to get photos for comment",
						"comment_id", comments[len(comments)-1].ID,
						"error", err,
					)
				}
			}

			if len(comments[len(comments)-1].Videos) > 0 {
				err = c.fillVideos(ctx, &comments[len(comments)-1].Videos)
				if err != nil {
					c.logger.Error("failed to get videos for comment",
						"comment_id", comments[len(comments)-1].ID,
						"error", err,
					)
				}
			}
		}
	}

	for i := range comments {
		err := c.fillReplies(ctx, authorID, postID, &comments[i])
		if err != nil {
			c.logger.Error("failed to fill replies", "error", err)
			return nil, err
		}
	}

	return &comments, nil
}

func (c *Client) fillStats(ctx context.Context, post *domain.Post) error {
	resp, err := c.doVkApiRequest(ctx, METHOD_GET_POST_STATS, map[string]any{
		"owner_id": post.OwnerID,
		"post_ids": post.ID,
	})
	if err != nil {
		return err
	}

	response := vkGetPostStatsResponse{}
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return err
	}
	if response.Error.IsNotNull(ctx, c.logger) {
		return errors.New(response.Error.Message)
	}
	if len(response.Response) == 0 ||
		(response.Response[0].Hide == 0 &&
			response.Response[0].JoinGroup == 0 &&
			response.Response[0].ReachSubscribers == 0 &&
			response.Response[0].ReachTotal == 0 &&
			response.Response[0].ToGroup == 0 &&
			response.Response[0].Unsubscribe == 0) {
		return ERROR_NO_STATS
	}

	post.Stats = response.Response[0].toDomain(post.Stats)

	return nil
}

// fillReplies implements DFS
func (c *Client) fillReplies(ctx context.Context, authorID, postID int, comment *domain.Comment) error {
	for offset := 0; ; offset += 10 {
		resp, err := c.doVkApiRequest(ctx, METHOD_GET_COMMENTS, map[string]any{
			"owner_id":   authorID,
			"post_id":    postID,
			"comment_id": comment.ID,
			"need_likes": 1,

			"count":  10,
			"offset": offset,
		})
		if err != nil {
			return err
		}

		response := vkGetCommentsResponse{}
		err = json.Unmarshal(resp, &response)
		if err != nil {
			return err
		}
		if response.Error.IsNotNull(ctx, c.logger) {
			return errors.New(response.Error.Message)
		}
		if len(response.Response.Items) == 0 {
			break
		}
		if comment.Replies == nil {
			replies := make([]domain.Comment, 0)
			comment.Replies = replies
		}

		for _, reply := range response.Response.Items {
			newReply, err := reply.toDomain(*c.logger, authorID)
			if err != nil {
				return err
			}
			comment.Replies = append(comment.Replies, newReply)

			if len(comment.Replies[len(comment.Replies)-1].Photos) > 0 {
				err = c.fillPhotos(ctx, &comment.Replies[len(comment.Replies)-1].Photos)
				if err != nil {
					c.logger.Error("failed to get photos for comment",
						"comment_id", comment.Replies[len(comment.Replies)-1].ID,
						"error", err,
					)
				}
			}

			c.logger.Debug("New comment",
				"Text", newReply.Text,
			)

			err = c.fillReplies(ctx, authorID, postID, &newReply)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
