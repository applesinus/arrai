package disk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"arrai/config/appEnv"
	"arrai/internal/domain"
	"arrai/internal/repository"
)

var (
	ERR_PATH_INVALID = errors.New("invalid path")
	ERR_PATH_UNSAFE  = errors.New("unsafe path")
)

type Client struct {
	mu  *sync.Mutex
	ctx context.Context

	appEnv *appEnv.AppEnv
	logger *slog.Logger

	basePath     string
	authorID     string
	providerName string
}

func New(ctx context.Context, logger *slog.Logger, appEnv *appEnv.AppEnv, providerName string, authorID string) (repository.Repository, error) {
	if providerName == "" {
		return nil, repository.ERR_EMPTY_PROVIDER
	}

	if authorID == "" {
		return nil, repository.ERR_EMPTY_AUTHOR
	}

	basePath := appEnv.MustGet(repository.BASE_PATH_ENV_KEY)
	if basePath == "" {
		return nil, repository.ERR_EMPTY_BASE_PATH
	}

	if logger == nil {
		return nil, repository.ERR_NO_LOGGER
	}

	if appEnv == nil {
		return nil, repository.ERR_NO_APP_ENV
	}

	diskRepo := &Client{
		mu:     &sync.Mutex{},
		ctx:    ctx,
		appEnv: appEnv,
		logger: logger,

		basePath:     basePath,
		authorID:     authorID,
		providerName: providerName,
	}

	_, err := os.Stat(basePath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(basePath, 0755)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	// Provider directory
	providerPath := fmt.Sprintf("%s/%s", basePath, providerName)
	_, err = os.Stat(providerPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(providerPath, 0755)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	// Author directory
	authorPath := fmt.Sprintf("%s/%s", providerPath, authorID)
	_, err = os.Stat(authorPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(authorPath, 0755)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return repository.Repository(diskRepo), nil
}

// SAVE

func (c *Client) savePost(dirPath, filePath string, post domain.Post) (int, error) {
	if c.isPathExists(filePath) {
		return -1, repository.ERR_POST_EXISTS
	}

	photoDirPath := fmt.Sprintf("%s/%d", dirPath, post.ID)

	if len(post.Photos) > 0 {
		if !c.isPathExists(photoDirPath) {
			err := os.MkdirAll(photoDirPath, 0755)
			if err != nil {
				return -1, err
			}
		}

		for i, photo := range post.Photos {
			photoFilenames, err := c.savePhoto(photoDirPath, fmt.Sprintf("%d.jpg", i), photo)
			if err != nil {
				return -1, err
			}

			post.Photos[i].BigSize.Filename = photoFilenames[0]
			post.Photos[i].BigSize.Content = []byte{}

			post.Photos[i].SmallSize.Filename = photoFilenames[1]
			post.Photos[i].SmallSize.Content = []byte{}
		}
	}

	err := c.saveCommentsPhotos(photoDirPath, &post.Comments)
	if err != nil {
		return -1, err
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

func (c *Client) saveCommentsPhotos(dirPath string, comments *[]domain.Comment) error {
	for i, comment := range *comments {
		if len(comment.Photos) > 0 {
			if !c.isPathExists(fmt.Sprintf("%s/%d", dirPath, comment.ID)) {
				err := os.MkdirAll(fmt.Sprintf("%s/%d", dirPath, comment.ID), 0755)
				if err != nil {
					return err
				}
			}

			for j, photo := range comment.Photos {
				photoFilenames, err := c.savePhoto(fmt.Sprintf("%s/%d", dirPath, comment.ID), fmt.Sprintf("%d.jpg", j), photo)
				if err != nil {
					return err
				}

				comment.Photos[j].BigSize.Filename = photoFilenames[0]
				comment.Photos[j].BigSize.Content = []byte{}

				comment.Photos[j].SmallSize.Filename = photoFilenames[1]
				comment.Photos[j].SmallSize.Content = []byte{}
			}
		}

		err := c.saveCommentsPhotos(dirPath, &(*comments)[i].Replies)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) savePhoto(dirPath, filename string, photo domain.TwoSizesPhoto) ([]string, error) {
	filenames := make([]string, 2)

	name := filename
	if c.isPathExists(fmt.Sprintf("%s/%s", dirPath, name)) {
		return nil, repository.ERR_PHOTO_EXISTS
	}
	err := c.saveOneSizePhoto(dirPath, name, photo.BigSize)
	if err != nil {
		return nil, err
	}
	filenames[0] = name

	name = fmt.Sprintf("%s%s", repository.PHOTO_SMALL_PREFIX, filename)
	if c.isPathExists(fmt.Sprintf("%s/%s", dirPath, name)) {
		return nil, repository.ERR_PHOTO_EXISTS
	}
	err = c.saveOneSizePhoto(dirPath, name, photo.SmallSize)
	if err != nil {
		return nil, err
	}
	filenames[1] = name

	return filenames, nil
}

func (c *Client) saveOneSizePhoto(dirPath, filename string, photo domain.Photo) error {
	file, err := os.OpenFile(fmt.Sprintf("%s/%s", dirPath, filename), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write(photo.Content)

	return nil
}

func (c *Client) SavePost(post domain.Post) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.savePost(c.directoryPath(), c.filePath(post.ID), post)
}

func (c *Client) SavePosts(posts []domain.Post) (map[int]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	postIDs := make(map[int]int, len(posts))
	for _, post := range posts {
		id, err := c.savePost(c.directoryPath(), c.filePath(post.ID), post)
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

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return post, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&post)
	if err != nil {
		return post, err
	}

	photoDirPath := strings.TrimSuffix(filePath, ".json")

	if len(post.Photos) > 0 {
		for i := range post.Photos {
			err := c.readPhoto(photoDirPath, &post.Photos[i])
			if err != nil {
				return post, err
			}
		}
	}

	if len(post.Comments) > 0 {
		err = c.fillCommentsPhotos(photoDirPath, &post.Comments)
		if err != nil {
			return post, err
		}
	}

	return post, err
}

func (c *Client) fillCommentsPhotos(dirPath string, replies *[]domain.Comment) error {
	for i := range *replies {
		for j := range (*replies)[i].Photos {
			err := c.readPhoto(dirPath, &(*replies)[i].Photos[j])
			if err != nil {
				return err
			}
		}

		err := c.fillCommentsPhotos(dirPath, &(*replies)[i].Replies)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) readPhoto(dirPath string, photo *domain.TwoSizesPhoto) error {
	c.logger.Debug("Reading Photos",
		"dirPath", dirPath,
		"bigFilename", photo.BigSize.Filename,
		"smallFilename", photo.SmallSize.Filename,
	)

	bigPhoto, err := c.getOneSizePhoto(dirPath, photo.BigSize.Filename)
	if err != nil {
		return err
	}
	photo.BigSize.Content = bigPhoto

	smallPhoto, err := c.getOneSizePhoto(dirPath, photo.SmallSize.Filename)
	if err != nil {
		return err
	}
	photo.SmallSize.Content = smallPhoto

	return nil
}

func (c *Client) getOneSizePhoto(dirPath, filename string) ([]byte, error) {
	if !c.isPathExists(fmt.Sprintf("%s/%s", dirPath, filename)) {
		return nil, repository.ERR_PHOTO_NOT_FOUND
	}

	file, err := os.OpenFile(fmt.Sprintf("%s/%s", dirPath, filename), os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

func (c *Client) GetPost(postID int) (domain.Post, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getPost(c.filePath(postID))
}

func (c *Client) GetPosts(postIDs []int) ([]domain.Post, error) {
	posts := make([]domain.Post, len(postIDs))

	c.mu.Lock()
	defer c.mu.Unlock()

	for i, postID := range postIDs {
		post, err := c.getPost(c.filePath(postID))
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

func (c *Client) GetExistingPostIDs() ([]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getExistingPostIDs(c.directoryPath())
}

func (c *Client) GetAllPosts() ([]domain.Post, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	postIDs, err := c.getExistingPostIDs(c.directoryPath())
	if err != nil {
		return nil, err
	}

	posts := make([]domain.Post, len(postIDs))

	for i, postID := range postIDs {
		post, err := c.getPost(c.filePath(postID))
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

func (c *Client) UpdatePost(post domain.Post) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.updatePost(c.filePath(post.ID), post)
}

func (c *Client) UpdatePosts(posts []domain.Post) error {
	for _, post := range posts {
		err := c.updatePost(c.filePath(post.ID), post)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) UpdateOrSavePosts(posts []domain.Post) error {
	for _, post := range posts {
		err := c.updatePost(c.filePath(post.ID), post)
		if err != nil {
			if err == repository.ERR_POST_NOT_FOUND {
				_, err = c.savePost(c.directoryPath(), c.filePath(post.ID), post)
				if err != nil {
					return err
				}
			}
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

func (c *Client) DeletePost(postID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.deletePost(c.filePath(postID))
}

func (c *Client) clear(dirPath string) error {
	if !c.isPathExists(dirPath) {
		return nil
	}

	return os.RemoveAll(dirPath)
}

func (c *Client) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.clear(c.directoryPath())
}

// HELPERS

func (c *Client) filePath(postID int) string {
	return fmt.Sprintf("%s/%d.json", c.directoryPath(), postID)
}

func (c *Client) directoryPath() string {
	return fmt.Sprintf("%s/%s/%s", c.basePath, c.providerName, c.authorID)
}

func (c *Client) isPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (c *Client) revealInExplorer(path string) error {
	return exec.Command("explorer", path).Start()
}

func (c *Client) isPathValid(path string) error {
	if path == "" {
		return ERR_PATH_INVALID
	}

	if strings.ContainsRune(path, 0) {
		return ERR_PATH_INVALID
	}

	cleaned := filepath.Clean(path)

	if cleaned == "." && path != "." {
		return ERR_PATH_INVALID
	}

	if runtime.GOOS == "windows" {
		if strings.ContainsAny(path, `<>:"|?*`) {
			return ERR_PATH_INVALID
		}
	}

	if !c.isSafePath(c.basePath, cleaned) {
		return ERR_PATH_UNSAFE
	}

	return nil
}

func (c *Client) isSafePath(baseDir, userPath string) bool {
	finalPath := filepath.Join(baseDir, userPath)

	rel, err := filepath.Rel(baseDir, finalPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(rel, "..") && rel != ".."
}
