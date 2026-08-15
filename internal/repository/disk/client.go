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
	mu *sync.Mutex

	appEnv *appEnv.AppEnv
	logger *slog.Logger

	basePath     string
	authorID     string
	providerName string
}

func New(logger *slog.Logger, appEnv *appEnv.AppEnv, providerName string, authorID string) (repository.Repository, error) {
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
	if c.isPathExist(filePath) {
		return -1, repository.ERR_POST_EXISTS
	}
	if post.ID == -1 {
		return -1, domain.ERROR_NO_POST_ID
	}

	photoDirPath := fmt.Sprintf("%s/%d", dirPath, post.ID)

	if len(post.Photos) > 0 {
		if !c.isPathExist(photoDirPath) {
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

			post.Photos[i].Self.Filename = photoFilenames[0]
			post.Photos[i].Self.Content = []byte{}

			post.Photos[i].Preview.Filename = photoFilenames[1]
			post.Photos[i].Preview.Content = []byte{}
		}
	}

	if len(post.Videos) > 0 {
		err := c.createDirIfNotExist(dirPath, post.ID)
		if err != nil {
			return -1, err
		}

		for j, video := range post.Videos {
			videoFilename, previewFilename, err := c.saveVideo(
				fmt.Sprintf("%s/%d", dirPath, post.ID),
				fmt.Sprintf("%d.mp4", j),
				fmt.Sprintf("%sv%d.jpg", repository.PREVIEW_PREFIX, j),
				video,
			)
			if err != nil {
				if videoFilename == "" {
					return -1, err
				}
				c.logger.Error("Failed to save preview for video",
					"post_id", post.ID,
					"video", videoFilename,
					"error", err,
				)
			}

			post.Videos[j].Filename = videoFilename
			post.Videos[j].Content = []byte{}

			post.Videos[j].Preview.Filename = previewFilename
			post.Videos[j].Preview.Content = []byte{}
		}
	}

	err := c.saveCommentsAttachments(photoDirPath, &post.Comments)
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

func (c *Client) saveCommentsAttachments(dirPath string, comments *[]domain.Comment) error {
	for i, comment := range *comments {
		if len(comment.Photos) > 0 {
			err := c.createDirIfNotExist(dirPath, comment.ID)
			if err != nil {
				return err
			}

			for j, photo := range comment.Photos {
				photoFilenames, err := c.savePhoto(fmt.Sprintf("%s/%d", dirPath, comment.ID), fmt.Sprintf("p%d.jpg", j), photo)
				if err != nil {
					return err
				}

				comment.Photos[j].Self.Filename = photoFilenames[0]
				comment.Photos[j].Self.Content = []byte{}

				comment.Photos[j].Preview.Filename = photoFilenames[1]
				comment.Photos[j].Preview.Content = []byte{}
			}
		}

		if len(comment.Videos) > 0 {
			err := c.createDirIfNotExist(dirPath, comment.ID)
			if err != nil {
				return err
			}

			for j, video := range comment.Videos {
				videoFilename, previewFilename, err := c.saveVideo(
					fmt.Sprintf("%s/%d", dirPath, comment.ID),
					fmt.Sprintf("v%d.mp4", j),
					fmt.Sprintf("%sv%d.jpg", repository.PREVIEW_PREFIX, j),
					video,
				)
				if err != nil {
					if videoFilename == "" {
						return err
					}
					c.logger.Error("Failed to save preview for video",
						"comment_id", comment.ID,
						"video", videoFilename,
						"error", err,
					)
				}

				comment.Videos[j].Filename = videoFilename
				comment.Videos[j].Content = []byte{}

				comment.Videos[j].Preview.Filename = previewFilename
				comment.Videos[j].Preview.Content = []byte{}
			}
		}

		err := c.saveCommentsAttachments(dirPath, &(*comments)[i].Replies)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) createDirIfNotExist(parentPath string, dirName any) error {
	if !c.isPathExist(fmt.Sprintf("%s/%v", parentPath, dirName)) {
		err := os.MkdirAll(fmt.Sprintf("%s/%v", parentPath, dirName), 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) saveVideo(dirPath, videoFilename string, previewFilename string, video domain.Video) (string, string, error) {
	// saving video itself
	file, err := os.OpenFile(fmt.Sprintf("%s/%s", dirPath, videoFilename), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	file.Write(video.Content)

	// saving preview
	err = c.savePicture(dirPath, previewFilename, video.Preview)
	if err != nil {
		return videoFilename, "", err
	}

	return videoFilename, previewFilename, nil
}

func (c *Client) savePhoto(dirPath, filename string, photo domain.Photo) ([]string, error) {
	filenames := make([]string, 2)

	name := filename
	if c.isPathExist(fmt.Sprintf("%s/%s", dirPath, name)) {
		return nil, repository.ERR_PHOTO_EXISTS
	}
	err := c.savePicture(dirPath, name, photo.Self)
	if err != nil {
		return nil, err
	}
	filenames[0] = name

	name = fmt.Sprintf("%s%s", repository.PREVIEW_PREFIX, filename)
	if c.isPathExist(fmt.Sprintf("%s/%s", dirPath, name)) {
		return nil, repository.ERR_PHOTO_EXISTS
	}
	err = c.savePicture(dirPath, name, photo.Preview)
	if err != nil {
		return nil, err
	}
	filenames[1] = name

	return filenames, nil
}

func (c *Client) savePicture(dirPath, filename string, photo domain.Picture) error {
	file, err := os.OpenFile(fmt.Sprintf("%s/%s", dirPath, filename), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write(photo.Content)

	return nil
}

func (c *Client) SavePost(ctx context.Context, post domain.Post) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.savePost(c.directoryPath(), c.filePath(post.ID), post)
}

func (c *Client) SavePosts(ctx context.Context, posts []domain.Post) (map[int]int, error) {
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
			err := c.getPhoto(photoDirPath, &post.Photos[i])
			if err != nil {
				return post, err
			}
		}
	}

	if len(post.Videos) > 0 {
		for i := range post.Videos {
			err := c.getVideo(photoDirPath, &post.Videos[i])
			if err != nil {
				return post, err
			}
		}
	}

	if len(post.Comments) > 0 {
		err = c.fillCommentsAttachments(photoDirPath, &post.Comments)
		if err != nil {
			return post, err
		}
	}

	return post, err
}

func (c *Client) fillCommentsAttachments(dirPath string, replies *[]domain.Comment) error {
	for i := range *replies {
		for j := range (*replies)[i].Photos {
			err := c.getPhoto(dirPath, &(*replies)[i].Photos[j])
			if err != nil {
				return err
			}
		}

		for j := range (*replies)[i].Videos {
			err := c.getVideo(dirPath, &(*replies)[i].Videos[j])
			if err != nil {
				return err
			}
		}

		err := c.fillCommentsAttachments(dirPath, &(*replies)[i].Replies)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) getVideo(dirPath string, video *domain.Video) error {
	c.logger.Debug("Reading Videos",
		"directory_path", dirPath,
		"video_filename", video.Filename,
		"preview_filename", video.Preview.Filename,
	)

	videoFile, err := c.readFile(dirPath, video.Filename)
	if err != nil {
		return err
	}
	video.Content = videoFile

	previewFile, err := c.readFile(dirPath, video.Preview.Filename)
	if err != nil {
		return err
	}
	video.Preview.Content = previewFile

	return nil
}

func (c *Client) getPhoto(dirPath string, photo *domain.Photo) error {
	c.logger.Debug("Reading Photos",
		"dirPath", dirPath,
		"bigFilename", photo.Self.Filename,
		"smallFilename", photo.Preview.Filename,
	)

	bigPhoto, err := c.readFile(dirPath, photo.Self.Filename)
	if err != nil {
		return err
	}
	photo.Self.Content = bigPhoto

	smallPhoto, err := c.readFile(dirPath, photo.Preview.Filename)
	if err != nil {
		return err
	}
	photo.Preview.Content = smallPhoto

	return nil
}

func (c *Client) readFile(dirPath, filename string) ([]byte, error) {
	if !c.isPathExist(fmt.Sprintf("%s/%s", dirPath, filename)) {
		return nil, repository.ERR_PHOTO_NOT_FOUND
	}

	file, err := os.OpenFile(fmt.Sprintf("%s/%s", dirPath, filename), os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

func (c *Client) GetPost(ctx context.Context, postID int) (domain.Post, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getPost(c.filePath(postID))
}

func (c *Client) GetPosts(ctx context.Context, postIDs []int) ([]domain.Post, error) {
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

	if !c.isPathExist(dirPath) {
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

func (c *Client) GetExistingPostIDs(ctx context.Context) ([]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getExistingPostIDs(c.directoryPath())
}

func (c *Client) GetAllPosts(ctx context.Context) ([]domain.Post, error) {
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
	if !c.isPathExist(filePath) {
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

func (c *Client) UpdatePost(ctx context.Context, post domain.Post) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.updatePost(c.filePath(post.ID), post)
}

func (c *Client) UpdatePosts(ctx context.Context, posts []domain.Post) error {
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
	if c.isPathExist(filePath) {
		return repository.ERR_POST_NOT_FOUND
	}

	return os.Remove(filePath)
}

func (c *Client) DeletePost(ctx context.Context, postID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.deletePost(c.filePath(postID))
}

func (c *Client) clear(dirPath string) error {
	if !c.isPathExist(dirPath) {
		return nil
	}

	return os.RemoveAll(dirPath)
}

func (c *Client) Clear(ctx context.Context) error {
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

func (c *Client) isPathExist(path string) bool {
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
