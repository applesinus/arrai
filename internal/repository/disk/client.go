package disk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

func New(ctx context.Context, logger *slog.Logger, appEnv *appEnv.AppEnv, providerType string, authorID string) repository.Repository {
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

	// Provider directory
	err = os.MkdirAll(fmt.Sprintf("%s/%s", basePath, providerType), 0755)
	if err != nil {
		logger.Error("failed to create provider path",
			"provider_type", providerType,
			"error", err,
		)
		panic(err)
	}

	// Author directory
	err = os.MkdirAll(fmt.Sprintf("%s/%s/%s", basePath, providerType, authorID), 0755)
	if err != nil {
		logger.Error("failed to create author path",
			"wall_id", authorID,
			"error", err,
		)
		panic(err)
	}

	return repository.Repository(diskRepo)
}

// SAVE

func (c *Client) savePost(filePath string, post domain.Post) (int, error) {
	if c.isPathExists(filePath) {
		return -1, repository.ERR_POST_EXISTS
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return -1, err
	}
	defer file.Close()

	c.logger.Debug("Opened file")

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

func (c *Client) SavePost(serviceName, authorID string, post domain.Post) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.savePost(c.filePath(serviceName, authorID, post.ID), post)
}

func (c *Client) SavePosts(serviceName, authorID string, posts []domain.Post) (map[int]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	postIDs := make(map[int]int, len(posts))
	for _, post := range posts {
		id, err := c.savePost(c.filePath(serviceName, authorID, post.ID), post)
		if err != nil {
			return nil, err
		}

		postIDs[id] = post.ID
	}

	return postIDs, nil
}

// GET

func (c *Client) getPost(filePath string) (domain.Post, error) {
	post := domain.Post{}

	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return post, repository.ERR_POST_NOT_FOUND
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return post, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&post)

	return post, err
}

func (c *Client) GetPost(serviceName, authorID string, postID int) (domain.Post, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getPost(c.filePath(serviceName, authorID, postID))
}

func (c *Client) GetPosts(serviceName, authorID string, postIDs []int) ([]domain.Post, error) {
	posts := make([]domain.Post, len(postIDs))

	c.mu.Lock()
	defer c.mu.Unlock()

	for i, postID := range postIDs {
		post, err := c.getPost(c.filePath(serviceName, authorID, postID))
		if err != nil {
			return nil, err
		}

		posts[i] = post
	}

	return posts, nil
}

// GET all

func (c *Client) getExistingPostIDs(dirPath string) ([]int, error) {
	postIDs := make([]int, 0)

	if !c.isPathExists(dirPath) {
		return postIDs, nil
	}

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		filename := file.Name()
		c.logger.Debug("File found",
			"filename", filename,
		)

		id, err := strconv.Atoi(strings.TrimSuffix(filename, ".json"))
		if err != nil {
			return nil, err
		}

		postIDs = append(postIDs, id)
	}

	return postIDs, nil
}

func (c *Client) GetExistingPostIDs(serviceName, authorID string) ([]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getExistingPostIDs(c.directoryPath(serviceName, authorID))
}

func (c *Client) GetAllPosts(serviceName, authorID string) ([]domain.Post, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	postIDs, err := c.getExistingPostIDs(c.directoryPath(serviceName, authorID))
	if err != nil {
		return nil, err
	}

	posts := make([]domain.Post, len(postIDs))

	for i, postID := range postIDs {
		post, err := c.getPost(c.filePath(serviceName, authorID, postID))
		if err != nil {
			return nil, err
		}

		posts[i] = post
	}

	return posts, nil
}

// UPDATE

func (c *Client) updatePost(filePath string, post domain.Post) error {
	if !c.isPathExists(filePath) {
		return repository.ERR_POST_NOT_FOUND
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonBytes, err := json.MarshalIndent(post, "", "  ")
	if err != nil {
		return err
	}

	_, err = file.Write(jsonBytes)
	return err
}

func (c *Client) UpdatePost(serviceName, authorID string, post domain.Post) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.updatePost(c.filePath(serviceName, authorID, post.ID), post)
}

func (c *Client) UpdatePosts(serviceName, authorID string, posts []domain.Post) error {
	for _, post := range posts {
		err := c.updatePost(c.filePath(serviceName, authorID, post.ID), post)
		if err != nil {
			return err
		}
	}

	return nil
}

// DELETE

func (c *Client) deletePost(filePath string) error {
	if c.isPathExists(filePath) {
		return repository.ERR_POST_NOT_FOUND
	}

	return os.Remove(filePath)
}

func (c *Client) DeletePost(serviceName, authorID string, postID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.deletePost(c.filePath(serviceName, authorID, postID))
}

func (c *Client) clear(dirPath string) error {
	if !c.isPathExists(dirPath) {
		return nil
	}

	return os.RemoveAll(dirPath)
}

func (c *Client) Clear(serviceName, authorID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.clear(c.directoryPath(serviceName, authorID))
}

// HELPERS

func (c *Client) filePath(serviceName, authorID string, postID int) string {
	return fmt.Sprintf("%s/%d.json", c.directoryPath(serviceName, authorID), postID)
}

func (c *Client) directoryPath(serviceName, authorID string) string {
	return fmt.Sprintf("%s/%s/%s", c.basePath, serviceName, authorID)
}

func (c *Client) isPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (c *Client) revealInExplorer(path string) error {
	return exec.Command("explorer", path).Start()
}
