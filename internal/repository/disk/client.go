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
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"arrai/config/appEnv"
	"arrai/internal/domain"
	"arrai/internal/repository"
)

var (
	ERR_PATH_INVALID                  = errors.New("invalid path")
	ERR_PATH_UNSAFE                   = errors.New("unsafe path")
	ERR_UNKNOWN_TYPE_WITH_ATTACHMENTS = errors.New("unknown type with attachments")

	ERR_NO_ENTITIES_GIVEN = errors.New("no entities given")
)

type Client struct {
	mu *sync.Mutex

	appEnv *appEnv.AppEnv
	logger *slog.Logger

	basePath     string
	authorID     string
	providerName string
}

// Current version of DiskRepo does not support multi-threaded access
// TODO
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

// Interface for entities with attachments e.g. posts, comments
type haveAttachments interface {
	domain.Post | domain.Comment
}

// SAVE

// SavePost saves one post
func (c *Client) SavePost(ctx context.Context, post domain.Post) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.savePost(c.buildAuthorPath(), c.buildPostFilePath(post.ID), post)
}

// SavePosts saves multiple posts to the disk repo
//
// Ranges over the slice and try to save each post. Not returning an error if one of them fails until the end
func (c *Client) SavePosts(ctx context.Context, posts []domain.Post) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error = nil
	for _, post := range posts {
		saveError := c.savePost(c.buildAuthorPath(), c.buildPostFilePath(post.ID), post)
		if err != nil {
			err = errors.Join(err, fmt.Errorf("post_%d", post.ID), saveError)
			continue
		}
	}

	return err
}

// savePost saves one post to the disk repo
func (c *Client) savePost(dirPath, filePath string, post domain.Post) error {
	if post.ID == -1 {
		return domain.ERROR_NO_POST_ID
	}
	if c.isPathExist(filePath) {
		return repository.ERR_POST_EXISTS
	}

	// creating a directory for the post attachments
	err := c.createDirIfNotExist(dirPath, post.ID)
	if err != nil {
		return err
	}

	// saving all attachments including attachments in comments recursively
	err = saveAttachments(c, fmt.Sprintf("%s/%d", dirPath, post.ID), &[]domain.Post{post})
	if err != nil {
		if errors.Is(err, ERR_NO_ENTITIES_GIVEN) {
			c.logger.Warn(err.Error(),
				"entity_type", "domain.Post",
				"entity_id", post.ID,
			)
		} else {
			return err
		}
	}

	// checking if the attachment folder is empty and removing if it is
	if c.isDirEmpty(fmt.Sprintf("%s/%d", dirPath, post.ID)) {
		err := os.Remove(fmt.Sprintf("%s/%d", dirPath, post.ID))
		if err != nil {
			return err
		}
	}

	// saving the post into JSON
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
	if err != nil {
		return err
	}

	return nil
}

// saveAttachments saves all attachments recursively including attachments in comments/replies
func saveAttachments[T haveAttachments](repoClient *Client, dirPath string, entitiesWithAttachments *[]T) error {
	if entitiesWithAttachments == nil {
		return ERR_NO_ENTITIES_GIVEN
	}

	for _, entity := range *entitiesWithAttachments {
		// dealing with generic type
		var (
			id       *int
			photos   *[]domain.Photo
			videos   *[]domain.Video
			comments *[]domain.Comment
			preffix  string
		)

		switch t := any(entity).(type) {
		case domain.Post:
			id = &t.ID
			photos = &t.Photos
			videos = &t.Videos
			comments = &t.Comments
			preffix = "post"

		case domain.Comment:
			id = &t.ID
			photos = &t.Photos
			videos = &t.Videos
			comments = &t.Replies
			preffix = "comment"

		default:
			return ERR_UNKNOWN_TYPE_WITH_ATTACHMENTS
		}

		// saving photos
		if photos != nil && len(*photos) > 0 {
			for j, photo := range *photos {
				photoFilename := fmt.Sprintf("%s%d_p%d.jpg", preffix, *id, j)
				previewFilename := fmt.Sprintf("%s%d_p%d%s.jpg", preffix, *id, j, repository.PREVIEW_SUFFIX)

				err := repoClient.savePhoto(
					dirPath,
					photoFilename,
					previewFilename,
					photo,
				)
				if err != nil {
					return err
				}

				// updating the original struct to contain filenames
				(*photos)[j].Self.Filename = photoFilename
				(*photos)[j].Preview.Filename = previewFilename

				//(*photos)[j].Self.Content = []byte{}
				//(*photos)[j].Preview.Content = []byte{}
			}
		}

		// saving videos
		// TODO content generic??? kinda violates DRY???
		if videos != nil && len(*videos) > 0 {
			for j, video := range *videos {
				videoFilename := fmt.Sprintf("%s%d_v%d.mp4", preffix, *id, j)
				previewFilename := fmt.Sprintf("%s%d_v%d%s.jpg", preffix, *id, j, repository.PREVIEW_SUFFIX)

				err := repoClient.saveVideo(
					dirPath,
					videoFilename,
					previewFilename,
					video,
				)
				if err != nil {
					return err
				}

				// updating the original struct to contain filenames
				(*videos)[j].Filename = videoFilename
				(*videos)[j].Preview.Filename = previewFilename

				//(*videos)[j].Content = []byte{}
				//(*videos)[j].Preview.Content = []byte{}
			}
		}

		// saving comments' attachments recursively
		if comments == nil || len(*comments) == 0 {
			continue
		}
		err := saveAttachments(repoClient, dirPath, comments)
		if err != nil {
			if errors.Is(err, ERR_NO_ENTITIES_GIVEN) {
				repoClient.logger.Warn(err.Error(),
					"entity_type", reflect.TypeOf(entity),
					"entity_id", *id,
				)
			} else {
				return err
			}
		}
	}

	return nil
}

func (c *Client) saveVideo(dirPath, videoFilename, previewFilename string, video domain.Video) error {
	// saving video itself
	file, err := os.OpenFile(fmt.Sprintf("%s/%s", dirPath, videoFilename), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write(video.Content)

	// saving preview
	err = c.savePicture(dirPath, previewFilename, video.Preview)
	if err != nil {
		c.logger.Warn("can't save preview",
			"video", videoFilename,
			"error", err,
		)
	}

	return nil
}

func (c *Client) savePhoto(dirPath, photoFilename, previewFilename string, photo domain.Photo) error {
	// saving photo itself
	if c.isPathExist(fmt.Sprintf("%s/%s", dirPath, photoFilename)) {
		return fmt.Errorf("%s for %s", repository.ERR_PHOTO_EXISTS.Error(), photoFilename)
	}
	err := c.savePicture(dirPath, photoFilename, photo.Self)
	if err != nil {
		return err
	}

	// saving preview
	if c.isPathExist(fmt.Sprintf("%s/%s", dirPath, previewFilename)) {
		return fmt.Errorf("%s for %s", repository.ERR_PHOTO_EXISTS.Error(), previewFilename)
	}
	err = c.savePicture(dirPath, previewFilename, photo.Preview)
	if err != nil {
		c.logger.Warn("can't save preview",
			"photo", photoFilename,
			"preview", previewFilename,
			"error", err,
		)
	}

	return nil
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

// GET

// GetPost returns one post or an error if something failed
func (c *Client) GetPost(ctx context.Context, postID int) (domain.Post, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getPost(c.buildPostFilePath(postID))
}

// GetPosts returns multiple posts or an error if something failed
//
// Ranges over the slice and try to get each post. Not returning an error if one of them fails until the end
func (c *Client) GetPosts(ctx context.Context, postIDs []int) (map[int]domain.Post, error) {
	posts := make(map[int]domain.Post, len(postIDs))
	var err error = nil

	c.mu.Lock()
	defer c.mu.Unlock()

	for i, postID := range postIDs {
		post, getError := c.getPost(c.buildPostFilePath(postID))
		if err != nil {
			err = errors.Join(err, fmt.Errorf("post_%d", postID), getError)
			continue
		}

		posts[i] = post
	}

	return posts, err
}

func (c *Client) getPost(filePath string) (domain.Post, error) {
	post := domain.Post{}

	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return post, repository.ERR_POST_NOT_FOUND
	}

	// reading post itself
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return post, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&post)
	if err != nil {
		return post, err
	}

	// filling attachments' content
	attachmentsDirPath := strings.TrimSuffix(filePath, ".json")
	err = fillAttachments(c, attachmentsDirPath, &[]domain.Post{post})
	if err != nil {
		if errors.Is(err, ERR_NO_ENTITIES_GIVEN) {
			c.logger.Warn(err.Error(),
				"entity_type", "post",
				"entity_id", post.ID,
			)
		} else {
			return post, err
		}
	}

	return post, err
}

// fillAttachments fills given attachments recursively including attachments in comments/replies
//
// it works with an existing post/comments slice and modifies it in-place
func fillAttachments[T haveAttachments](repoClient *Client, dirPath string, entitiesWithAttachments *[]T) error {
	if len(*entitiesWithAttachments) == 0 {
		return ERR_NO_ENTITIES_GIVEN
	}

	for _, entity := range *entitiesWithAttachments {
		// dealing with generic type
		var (
			id       *int
			photos   *[]domain.Photo
			videos   *[]domain.Video
			comments *[]domain.Comment
		)

		switch t := any(entity).(type) {
		case domain.Post:
			id = &t.ID
			photos = &t.Photos
			videos = &t.Videos
			comments = &t.Comments
		case domain.Comment:
			id = &t.ID
			photos = &t.Photos
			videos = &t.Videos
			comments = &t.Replies

		default:
			return ERR_UNKNOWN_TYPE_WITH_ATTACHMENTS
		}

		// filling photos
		for j := range *photos {
			err := repoClient.fillPhoto(dirPath, &(*photos)[j])
			if err != nil {
				return err
			}
		}

		// filling videos
		for j := range *videos {
			err := repoClient.fillVideo(dirPath, &(*videos)[j])
			if err != nil {
				return err
			}
		}

		// filling comments' attachments recursively
		if comments == nil || len(*comments) == 0 {
			continue
		}
		err := fillAttachments(repoClient, dirPath, comments)
		if err != nil {
			if errors.Is(err, ERR_NO_ENTITIES_GIVEN) {
				repoClient.logger.Warn(err.Error(),
					"entity_type", reflect.TypeOf(entity),
					"entity_id", *id,
				)
			} else {
				return err
			}
		}
	}

	return nil
}

// fillAttachments fills given video
//
// it works with an existing video and modifies it in-place
func (c *Client) fillVideo(dirPath string, video *domain.Video) error {
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

// fillAttachments fills given photo
//
// it works with an existing photo and modifies it in-place
func (c *Client) fillPhoto(dirPath string, photo *domain.Photo) error {
	photoItself, err := c.readFile(dirPath, photo.Self.Filename)
	if err != nil {
		return err
	}
	photo.Self.Content = photoItself

	photoPreview, err := c.readFile(dirPath, photo.Preview.Filename)
	if err != nil {
		return err
	}
	photo.Preview.Content = photoPreview

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

// GET all

// GetAllPosts returns all posts
//
// Ranges over the slice and try to get each post. Not returning an error if one of them fails until the end
func (c *Client) GetAllPosts(ctx context.Context) ([]domain.Post, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error = nil

	postIDs, err := c.getExistingPostIDs(c.buildAuthorPath())
	if err != nil {
		return nil, err
	}

	posts := make([]domain.Post, 0, len(postIDs))

	for _, postID := range postIDs {
		post, getError := c.getPost(c.buildPostFilePath(postID))
		if err != nil {
			err = errors.Join(err, fmt.Errorf("post_%d", postID), getError)
			continue
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// GetExistingPostIDs returns all existing post ids
//
// Ranges over the database and try to get each filename. Not returning an error if one of them fails until the end.
// The result may be not empty even if error is not nil - if some filenames were valid but other filenames couldn't be parsed.
func (c *Client) GetExistingPostIDs(ctx context.Context) ([]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getExistingPostIDs(c.buildAuthorPath())
}

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

		id, parseErr := strconv.Atoi(strings.TrimSuffix(filename, ".json"))
		if parseErr != nil {
			err = errors.Join(err, fmt.Errorf("file_%s", filename), parseErr)
			continue
		}

		postIDs = append(postIDs, id)
	}

	return postIDs, err
}

// UPDATE

// UpdatePost updates one post
func (c *Client) UpdatePost(ctx context.Context, post domain.Post) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.updatePost(c.buildPostFilePath(post.ID), post)
}

// UpdatePosts updates multiple posts
//
// Ranges over the slice and try to update each post. Returning a combined error in the end of the range
func (c *Client) UpdatePosts(ctx context.Context, posts []domain.Post) error {
	var err error = nil

	for _, post := range posts {
		updateErr := c.updatePost(c.buildPostFilePath(post.ID), post)
		if err != nil {
			err = errors.Join(err, fmt.Errorf("post_%d", post.ID), updateErr)
			continue
		}
	}

	return err
}

// UpdateOrSavePost updates a post if it exists, otherwise saves it
func (c *Client) UpdateOrSavePost(ctx context.Context, post domain.Post) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.updatePost(c.buildPostFilePath(post.ID), post)
	if err != nil {
		if err == repository.ERR_POST_NOT_FOUND {
			return c.savePost(c.buildAuthorPath(), c.buildPostFilePath(post.ID), post)
		}
	}

	return err
}

// UpdateOrSavePosts updates a post if it exists, otherwise saves it
//
// Ranges over the slice and try to update or save each post. Returning a combined error in the end of the range
func (c *Client) UpdateOrSavePosts(posts []domain.Post) error {
	var err error = nil

	for _, post := range posts {
		updateErr := c.updatePost(c.buildPostFilePath(post.ID), post)
		if updateErr != nil {
			if updateErr == repository.ERR_POST_NOT_FOUND {
				saveErr := c.savePost(c.buildAuthorPath(), c.buildPostFilePath(post.ID), post)

				if saveErr != nil {
					err = errors.Join(err, fmt.Errorf("post_%d", post.ID), updateErr, saveErr)
					continue
				}
			}
		}
	}

	return err
}

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

// DELETE

// DeletePost deletes one post
func (c *Client) DeletePost(ctx context.Context, postID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.deletePost(postID)
}

// deletePost deletes one post hardly removing a file and a corresponding directory of attachments if it exists from disk
func (c *Client) deletePost(postID int) error {
	if !c.isPathExist(c.buildPostFilePath(postID)) {
		return repository.ERR_POST_NOT_FOUND
	}

	var err error = nil
	if c.isPathExist(c.buildPostAttachmentsDirPath(postID)) {
		err = os.Remove(c.buildPostAttachmentsDirPath(postID))
	}

	return errors.Join(err, os.Remove(c.buildPostFilePath(postID)))
}

// CLEAR

// Clear deletes all posts
func (c *Client) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.clear(c.buildAuthorPath())
}

// clear deletes all posts hardly removing a directory from disk
func (c *Client) clear(dirPath string) error {
	if !c.isPathExist(dirPath) {
		return nil
	}

	return os.RemoveAll(dirPath)
}

// HELPERS

// buildPostFilePath returns a string with presumable path to a file containing a post with .json extension
func (c *Client) buildPostFilePath(postID int) string {
	return fmt.Sprintf("%s/%d.json", c.buildAuthorPath(), postID)
}

// buildPostAttachmentsDirPath returns a string with presumable path to a directory containing attachments of a post
func (c *Client) buildPostAttachmentsDirPath(postID int) string {
	return fmt.Sprintf("%s/%d", c.buildAuthorPath(), postID)
}

// buildAuthorPath returns a string with presumable path to a directory containing posts of the author
func (c *Client) buildAuthorPath() string {
	return fmt.Sprintf("%s/%s/%s", c.basePath, c.providerName, c.authorID)
}

func (c *Client) isPathExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (c *Client) isDirEmpty(path string) bool {
	files, err := os.ReadDir(path)
	if err != nil {
		return true
	}

	return len(files) == 0
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

func (c *Client) createDirIfNotExist(parentPath string, dirName any) error {
	if !c.isPathExist(fmt.Sprintf("%s/%v", parentPath, dirName)) {
		err := os.MkdirAll(fmt.Sprintf("%s/%v", parentPath, dirName), 0755)
		if err != nil {
			return err
		}
	}

	return nil
}
