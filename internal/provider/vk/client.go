package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/time/rate"

	"arrai/config/appEnv"
	"arrai/internal/domain"
)

type Client struct {
	ctx    context.Context
	logger *slog.Logger
	env    *appEnv.AppEnv

	accessToken string
	limiter     *rate.Limiter
	httpClient  *http.Client

	debugMode bool
}

func NewClient(ctx context.Context, logger *slog.Logger, appEnv *appEnv.AppEnv, accessToken string) *Client {
	return &Client{
		ctx:    ctx,
		logger: logger,
		env:    appEnv,

		accessToken: accessToken,
		limiter:     rate.NewLimiter(rate.Limit(appEnv.GetIntOrDefault("VK_MAX_RPS", 3)), 1),
		httpClient:  http.DefaultClient,

		debugMode: appEnv.GetBoolOrDefault("DEBUG_MODE", false),
	}
}

func (c *Client) createLink(method string, params map[string]any) string {
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

func (c *Client) doVkApiRequest(method string, params map[string]any) ([]byte, error) {
	err := c.limiter.Wait(c.ctx)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Get(c.createLink(method, params))
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

func (c *Client) getWallOwnerIntID(wallID string) (int, error) {
	resp, err := c.doVkApiRequest(METHOD_RESOLVE_NAME, map[string]any{"screen_name": wallID})
	if err != nil {
		return 0, err
	}

	response := vkResolveNameResponse{}
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return 0, err
	}
	if response.Response.ID == 0 {
		return 0, fmt.Errorf("failed to resolve wall owner ID: %s", wallID)
	}

	return response.Response.ID, nil
}

func (c *Client) GetWall(wallID string) (*[]domain.Post, error) {
	wallOwnerId, err := c.getWallOwnerIntID(wallID)
	if err != nil {
		return nil, err
	}
	wallOwnerId *= -1

	// Getting all posts
	posts := make([]Post, 0)

	for offset := 0; ; offset += 100 {
		resp, err := c.doVkApiRequest(METHOD_GET_WALL, map[string]any{
			"domain": wallID,
			"count":  100,
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

		if len(response.Response.Items) == 0 {
			break
		}

		for _, post := range response.Response.Items {
			newPost, err := post.toPost()
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
	}

	// Getting comments for all posts
	for i, post := range posts {
		posts[i].Comments, err = c.getComments(wallOwnerId, post.ID)
		if err != nil {
			c.logger.Error("failed to get comments for post",
				"post_id", post.ID,
				"error", err,
			)
		}

		if c.debugMode {
			break
		}
	}

	// Converting to domain
	domainPosts := make([]domain.Post, 0)
	for _, post := range posts {
		domainPost, err := post.ToDomain()
		if err != nil {
			return nil, err
		}

		c.logger.Debug("comments", "post_id", domainPost.ID, "comments", domainPost.Comments)
		domainPosts = append(domainPosts, domainPost)

		if c.debugMode {
			return &domainPosts, nil
		}
	}

	return &domainPosts, nil
}

func (c *Client) getComments(wallOwnerID, postID int) (*[]*Comment, error) {
	comments := make([]*Comment, 0)

	for offset := 0; ; offset += 10 {
		resp, err := c.doVkApiRequest(METHOD_GET_COMMENTS, map[string]any{
			"owner_id":   wallOwnerID,
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
		if len(response.Response.Items) == 0 {
			break
		}

		for _, comment := range response.Response.Items {
			newComment := comment.toComment()
			comments = append(comments, &newComment)
		}
	}

	for _, comment := range comments {
		err := c.fillReplies(wallOwnerID, postID, comment)
		if err != nil {
			c.logger.Error("failed to fill replies", "error", err)
			return nil, err
		}
	}

	return &comments, nil
}

// fillReplies implements DFS
func (c *Client) fillReplies(wallOwnerID, postID int, comment *Comment) error {
	for offset := 0; ; offset += 10 {
		resp, err := c.doVkApiRequest(METHOD_GET_COMMENTS, map[string]any{
			"owner_id":   wallOwnerID,
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
		if len(response.Response.Items) == 0 {
			break
		}
		if comment.Replies == nil {
			replies := make([]*Comment, 0)
			comment.Replies = &replies
		}

		for _, reply := range response.Response.Items {
			newReply := reply.toComment()
			*comment.Replies = append(*comment.Replies, &newReply)

			c.logger.Debug("New comment",
				"Text", newReply.Text,
			)

			err = c.fillReplies(wallOwnerID, postID, &newReply)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
