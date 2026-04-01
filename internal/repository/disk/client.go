package disk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"arrai/config/appEnv"
	"arrai/internal/domain"
	"arrai/internal/repository"
)

type Client struct {
	mu  *sync.Mutex
	ctx context.Context

	appEnv *appEnv.AppEnv
	logger *slog.Logger

	basePath string
}

func New(ctx context.Context, logger *slog.Logger, appEnv *appEnv.AppEnv, wallID string) repository.WallRepository {
	basePath := appEnv.MustGet("DISK_REPO_BASE_PATH")

	diskRepo := &Client{
		mu:     &sync.Mutex{},
		ctx:    ctx,
		appEnv: appEnv,
		logger: logger,

		basePath: basePath,
	}

	_, err := os.Stat(basePath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(basePath, 0755)
		if err != nil {
			logger.Error("failed to create disk repo base path",
				"error", err,
			)
			panic(err)
		}
	} else if err != nil {
		logger.Error("failed to create disk repo base path",
			"error", err,
		)
		panic(err)
	}

	err = os.MkdirAll(fmt.Sprintf("%s/%s", basePath, wallID), 0755)
	if err != nil {
		logger.Error("failed to create wall path",
			"wall_id", wallID,
			"error", err,
		)
		panic(err)
	}

	return repository.WallRepository(diskRepo)
}

func (c *Client) openFolder(path string) error {
	return exec.Command("explorer", path).Start()
}

func (c *Client) SavePost(serviceName, authorID string, post domain.Post) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	filePath := c.filePath(serviceName, authorID, post.ID)

	_, err := os.Stat(filePath)
	if !os.IsNotExist(err) {
		return -1, repository.ERR_POST_EXISTS
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return -1, err
	}
	defer file.Close()

	jsonBytes, err := json.MarshalIndent(post, "", "  ")
	if err != nil {
		return -1, err
	}

	_, err = file.Write(jsonBytes)
	if err != nil {
		return -1, err
	}

	return post.ID, nil
}

func (c *Client) SavePosts(serviceName, authorID string, posts []domain.Post) (map[int]int, error) {
	postIDs := make(map[int]int, len(posts))
	for _, post := range posts {
		id, err := c.SavePost(serviceName, authorID, post)
		if err != nil {
			return nil, err
		}
		postIDs[id] = post.ID
	}

	return postIDs, nil
}

// TODO
func (c *Client) GetPost(serviceName, authorID string, postID int) (domain.Post, error) {
	return domain.Post{}, nil
}

// TODO
func (c *Client) GetPosts(serviceName, authorID string, postIDs []int) ([]domain.Post, error) {
	return nil, nil
}

// TODO
func (c *Client) GetAllPosts(serviceName, authorID string) ([]domain.Post, error) {
	return nil, nil
}

// TODO
func (c *Client) GetExistingPostIDs(serviceName, authorID string) ([]int, error) {
	return nil, nil
}

// TODO
func (c *Client) UpdatePost(serviceName, authorID string, post domain.Post) error {
	return nil
}

// TODO
func (c *Client) UpdatePosts(serviceName, authorID string, posts []domain.Post) error {
	return nil
}

// TODO
func (c *Client) DeletePost(serviceName, authorID string, postID int) error {
	return nil
}

func (c *Client) filePath(serviceName, authorID string, postID int) string {
	return fmt.Sprintf("%s/%s/%s/%d.json", c.basePath, serviceName, authorID, postID)
}
