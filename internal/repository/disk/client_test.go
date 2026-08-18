package disk_test

import (
	"arrai/config/appEnv"
	"arrai/internal/domain"
	"arrai/internal/repository"
	"arrai/internal/repository/disk"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// path constants
const (
	//testTempPath  = "./temp"
	testBasePaths = "testDiskRepo"
	testProvider  = "provider"
	testAuthor    = "author"
)

// value constants & vars
const (
	photoSelfPreffix = "photo"
	videoSelfPreffix = "video"
	previewPreffix   = "preview"
	photoUrl         = "url"
	photoFilename    = "filename"
)

var (
	postID       = 1
	ownerID      = 2
	creationTime = time.Now().Truncate(0)
	views        = 3
	reactions    = 4
	reposts      = 5
	text         = "text"
	photo1       = createMockPhoto("1")
	photo2       = createMockPhoto("2")
	video1       = createMockVideo("3")
	video2       = createMockVideo("4")
	comment1     = domain.Comment{
		ID:        6,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 7,
		Text:      "text",
		Photos:    []domain.Photo{},
		Videos:    []domain.Video{},
		Replies:   []domain.Comment{},
	}
	comment2 = domain.Comment{
		ID:        8,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 9,
		Text:      "text",
		Photos:    []domain.Photo{},
		Videos:    []domain.Video{},
		Replies:   []domain.Comment{},
	}
	commentWithOneEveryAttachment = domain.Comment{
		ID:        10,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 11,
		Text:      "text",
		Photos: []domain.Photo{
			photo1,
		},
		Videos: []domain.Video{
			video1,
		},
		Replies: []domain.Comment{},
	}
	commentWithManyEveryAttachment = domain.Comment{
		ID:        12,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 13,
		Text:      "text",
		Photos: []domain.Photo{
			photo1,
			photo2,
		},
		Videos: []domain.Video{
			video1,
			video2,
		},
		Replies: []domain.Comment{},
	}
	commentsThread = domain.Comment{
		ID:        14,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 15,
		Text:      "text",
		Photos:    []domain.Photo{},
		Videos:    []domain.Video{},
		Replies:   []domain.Comment{comment1, commentWithManyEveryAttachment},
	}
	post1 = domain.Post{
		ID:        postID,
		OwnerID:   ownerID,
		CreatedAt: creationTime,
		Views:     views,
		Reactions: reactions,
		Reposts:   reposts,
		Text:      text,
		Photos:    []domain.Photo{photo1},
		Videos:    []domain.Video{video1},
		Comments:  []domain.Comment{comment1},
	}
	post1Changed = domain.Post{
		ID:        postID,
		OwnerID:   ownerID,
		CreatedAt: creationTime,
		Views:     views + 1,
		Reactions: reactions + 1,
		Reposts:   reposts + 1,
		Text:      text + "1",
		Photos:    []domain.Photo{photo1},
		Videos:    []domain.Video{video1},
		Comments:  []domain.Comment{comment1},
	}
	post2 = domain.Post{
		ID:        postID + 1,
		OwnerID:   ownerID,
		CreatedAt: creationTime,
		Views:     views,
		Reactions: reactions,
		Reposts:   reposts,
		Text:      text,
		Photos:    []domain.Photo{photo2},
		Videos:    []domain.Video{video2},
		Comments:  []domain.Comment{comment2},
	}
)

// updateStatus type
type updateStatus string

var (
	old updateStatus = "old"
	new updateStatus = "new"
)

func TestClient_New(t *testing.T) {
	testSetup(t)

	// Creating test values
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	env := appEnv.New(logger)
	basePath := env.MustGet(repository.BASE_PATH_ENV_KEY)

	// Test cases
	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		provider string
		author   string
		logger   *slog.Logger
		appEnv   *appEnv.AppEnv

		dirExpected     string
		repoNilExpected bool
		errExpected     error
	}{
		"base": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			provider: testProvider,
			author:   testAuthor,
			logger:   logger,
			appEnv:   env,

			dirExpected:     fmt.Sprintf("%s/%s", testProvider, testAuthor),
			repoNilExpected: false,
			errExpected:     nil,
		},

		"noLogger": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			provider: testProvider,
			author:   testAuthor,
			logger:   nil,
			appEnv:   env,

			dirExpected:     "",
			repoNilExpected: true,
			errExpected:     repository.ERR_NO_LOGGER,
		},
		"noAppEnv": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			provider: testProvider,
			author:   testAuthor,
			logger:   logger,
			appEnv:   nil,

			dirExpected:     "",
			repoNilExpected: true,
			errExpected:     repository.ERR_NO_APP_ENV,
		},

		"emptyProvider": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			provider: "",
			author:   testAuthor,
			logger:   logger,
			appEnv:   env,

			dirExpected:     "",
			repoNilExpected: true,
			errExpected:     repository.ERR_EMPTY_PROVIDER,
		},
		"existingProviderDir": {
			setupFunc: func() {
				err := os.MkdirAll(fmt.Sprintf("%s/%s", basePath, testProvider), 0755)
				if err != nil {
					t.Fatal(err)
				}
			},
			teardownFunc: func() {
				err := os.RemoveAll(fmt.Sprintf("%s/%s", basePath, testProvider))
				if err != nil {
					t.Fatal(err)
				}
			},

			provider: testProvider,
			author:   testAuthor,
			logger:   logger,
			appEnv:   env,

			dirExpected:     fmt.Sprintf("%s/%s", testProvider, testAuthor),
			repoNilExpected: false,
			errExpected:     nil,
		},

		"emptyAuthor": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			provider: testProvider,
			author:   "",
			logger:   logger,
			appEnv:   env,

			dirExpected:     "",
			repoNilExpected: true,
			errExpected:     repository.ERR_EMPTY_AUTHOR,
		},
		"existingAuthorDir": {
			setupFunc: func() {
				err := os.MkdirAll(fmt.Sprintf("%s/%s/%s", basePath, testProvider, testAuthor), 0755)
				if err != nil {
					t.Fatal(err)
				}
			},
			teardownFunc: func() {
				err := os.RemoveAll(fmt.Sprintf("%s/%s/%s", basePath, testProvider, testAuthor))
				if err != nil {
					t.Fatal(err)
				}
			},

			provider: testProvider,
			author:   testAuthor,
			logger:   logger,
			appEnv:   env,

			dirExpected:     fmt.Sprintf("%s/%s", testProvider, testAuthor),
			repoNilExpected: false,
			errExpected:     nil,
		},

		"emptyBasePath": {
			setupFunc: func() {
				t.Setenv(repository.BASE_PATH_ENV_KEY, "")
			},
			teardownFunc: func() {
				t.Setenv(repository.BASE_PATH_ENV_KEY, basePath)
			},

			provider: testProvider,
			author:   testAuthor,
			logger:   logger,
			appEnv:   env,

			dirExpected:     "",
			repoNilExpected: true,
			errExpected:     repository.ERR_EMPTY_BASE_PATH,
		},
	}

	// Running tests
	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			test.setupFunc()
			defer test.teardownFunc()

			repo, err := disk.New(test.logger, test.appEnv, test.provider, test.author)

			assert.ErrorIs(t, err, test.errExpected)
			assert.Equal(t, test.repoNilExpected, repo == nil)

			_, err = os.Stat(basePath + "/" + test.dirExpected)
			if os.IsNotExist(err) && test.dirExpected != "" {
				t.Errorf("Directory %s not created", basePath+test.dirExpected)
			}
		})
	}
}

// Depends on working New
func TestClient_SavePost(t *testing.T) {
	ctx, logger, env := testSetup(t)

	// Test cases
	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		post *domain.Post

		expextedErr error
	}{
		// Success cases
		"postWithTextSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments:  []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithOnePhotoSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos: []domain.Photo{
					photo1,
				},
				Videos:   []domain.Video{},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithOneVideoSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos: []domain.Video{
					video1,
				},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithManyPhotosSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos: []domain.Photo{
					photo1,
					photo2,
				},
				Videos:   []domain.Video{},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithManyVideoSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos: []domain.Video{
					video1,
					video2,
				},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithOneCommentSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					comment1,
				},
			},

			expextedErr: nil,
		},
		"postWithManyCommentsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					comment1,
					comment2,
				},
			},

			expextedErr: nil,
		},
		"postWithOneEveryAttachmentInCommentSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					commentWithOneEveryAttachment,
				},
			},

			expextedErr: nil,
		},
		"postWithManyEveryAttachmentInCommentSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					commentWithManyEveryAttachment,
				},
			},

			expextedErr: nil,
		},
		"postWithCommentsThreadSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					commentsThread,
				},
			},

			expextedErr: nil,
		},

		// Errors cases
		"noPostID": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: &domain.Post{
				ID:        -1,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments:  []domain.Comment{},
			},

			expextedErr: domain.ERROR_NO_POST_ID,
		},
		// TODO fail cases
		// TODO concurrency cases
	}

	// Running tests
	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			// clearing test data after each test
			t.Cleanup(func() {
				clearTestData(t, env.MustGet(repository.BASE_PATH_ENV_KEY))
			})

			client, err := disk.New(logger, env, testProvider, testAuthor)
			if err != nil {
				t.Fatal(err)
			}

			test.setupFunc()
			defer test.teardownFunc()

			// saving one post
			gotErr := client.SavePost(ctx, test.post)
			assert.ErrorIs(t, gotErr, test.expextedErr)

			// validating saved post
			if gotErr == nil {
				validateSavedPost(t, *env, *test.post, test.post.ID)
			}
		})
	}
}

// Depends on working New
//
// Does not double SavePost cases, tests only slice managing
func TestClient_SavePosts(t *testing.T) {
	ctx, logger, env := testSetup(t)

	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		posts *map[int]domain.Post

		expextedErr error
	}{
		// Success cases
		"noPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: &map[int]domain.Post{},

			expextedErr: nil,
		},
		"onePostSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: &map[int]domain.Post{
				post1.ID: post1,
			},

			expextedErr: nil,
		},
		"manyPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: &map[int]domain.Post{
				post1.ID: post1,
				post2.ID: post2,
			},

			expextedErr: nil,
		},

		// Errors cases
		"nilSliceError": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: nil,

			expextedErr: repository.ERR_INVALID_ARGUMENT,
		},
		// TODO fail cases
		// TODO concurrency cases
	}

	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			// clearing test data after each test
			t.Cleanup(func() {
				clearTestData(t, env.MustGet(repository.BASE_PATH_ENV_KEY))
			})

			client, err := disk.New(logger, env, testProvider, testAuthor)
			if err != nil {
				t.Fatal(err)
			}

			test.setupFunc()
			defer test.teardownFunc()

			// saving multiple posts
			gotErr := client.SavePosts(ctx, test.posts)
			assert.ErrorIs(t, gotErr, test.expextedErr)

			// validating saved posts
			if gotErr == nil {
				for _, post := range *test.posts {
					validateSavedPost(t, *env, post, post.ID)
				}
			}
		})
	}
}

func validateSavedPost(t *testing.T, env appEnv.AppEnv, expectedPost domain.Post, id int) {
	fullPath := fmt.Sprintf("%s/%s/%s/%d.json", env.MustGet(repository.BASE_PATH_ENV_KEY), testProvider, testAuthor, id)

	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		t.Errorf("File with ID %d not created", id)
	} else {
		file, err := os.OpenFile(fullPath, os.O_RDONLY, 0644)
		if err != nil {
			t.Errorf("File with ID %d could not be opened", id)
		} else {
			func() {
				defer file.Close()

				var gotPost domain.Post
				err = json.NewDecoder(file).Decode(&gotPost)
				if err != nil {
					t.Errorf("File with ID %d could not be decoded", id)
				} else {
					assert.Equal(t, expectedPost, gotPost)
				}
			}()
		}
	}
}

// Depends on working New, SavePost
func TestClient_GetPost(t *testing.T) {
	ctx, logger, env := testSetup(t)

	// Test cases
	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		post domain.Post

		expextedErr error
	}{
		// Success cases
		"postWithTextSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments:  []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithOnePhotoSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos: []domain.Photo{
					photo1,
				},
				Videos:   []domain.Video{},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithOneVideoSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos: []domain.Video{
					video1,
				},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithManyPhotosSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos: []domain.Photo{
					photo1,
					photo2,
				},
				Videos:   []domain.Video{},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithManyVideoSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos: []domain.Video{
					video1,
					video2,
				},
				Comments: []domain.Comment{},
			},

			expextedErr: nil,
		},
		"postWithOneCommentSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					comment1,
				},
			},

			expextedErr: nil,
		},
		"postWithManyCommentsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					comment1,
					comment2,
				},
			},

			expextedErr: nil,
		},
		"postWithOneEveryAttachmentInCommentSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					commentWithOneEveryAttachment,
				},
			},

			expextedErr: nil,
		},
		"postWithManyEveryAttachmentInCommentSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					commentWithManyEveryAttachment,
				},
			},

			expextedErr: nil,
		},
		"postWithCommentsThreadSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        postID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.Photo{},
				Videos:    []domain.Video{},
				Comments: []domain.Comment{
					commentsThread,
				},
			},

			expextedErr: nil,
		},

		// TODO fail cases
		// TODO concurrency cases
	}

	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			// clearing test data after each test
			t.Cleanup(func() {
				clearTestData(t, env.MustGet(repository.BASE_PATH_ENV_KEY))
			})

			client, err := disk.New(logger, env, testProvider, testAuthor)
			if err != nil {
				t.Fatal(err)
			}

			test.setupFunc()
			defer test.teardownFunc()

			// saving post to prepare test
			err = client.SavePost(ctx, &test.post)
			if err != nil {
				t.Fatal(err)
			}

			// getting post
			gotPost, gotErr := client.GetPost(ctx, test.post.ID)
			assert.ErrorIs(t, gotErr, test.expextedErr)
			assert.Equal(t, test.post, gotPost)
		})
	}
}

// Depends on working New, SavePost
//
// Does not double GetPost cases, tests only slice managing
func TestClient_GetPosts(t *testing.T) {
	ctx, logger, env := testSetup(t)

	// Test cases
	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		posts map[int]domain.Post

		expextedErr error
	}{
		// Success cases
		"noPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: make(map[int]domain.Post),

			expextedErr: nil,
		},
		"onePostSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: map[int]domain.Post{
				post1.ID: post1,
			},

			expextedErr: nil,
		},
		"manyPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: map[int]domain.Post{
				post1.ID: post1,
				post2.ID: post2,
			},

			expextedErr: nil,
		},

		// Errors cases
		"nilSliceError": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: nil,

			expextedErr: repository.ERR_INVALID_ARGUMENT,
		},
		// TODO fail cases
		// TODO concurrency cases
	}

	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			// clearing test data after each test
			t.Cleanup(func() {
				clearTestData(t, env.MustGet(repository.BASE_PATH_ENV_KEY))
			})

			client, err := disk.New(logger, env, testProvider, testAuthor)
			if err != nil {
				t.Fatal(err)
			}

			test.setupFunc()
			defer test.teardownFunc()

			// saving posts to prepare test
			if test.posts != nil {
				err = client.SavePosts(ctx, &test.posts)
				if err != nil {
					t.Fatal(err)
				}
			}

			// preparing IDs slice
			var postIDs []int = nil
			if test.posts != nil {
				postIDs = make([]int, 0, len(test.posts))
				for _, post := range test.posts {
					postIDs = append(postIDs, post.ID)
				}
			}

			// getting multiple posts
			gotPosts, gotErr := client.GetPosts(ctx, postIDs)
			assert.ErrorIs(t, gotErr, test.expextedErr)
			assert.Equal(t, test.posts, gotPosts)
		})
	}
}

// Depends on working New, SavePost
func TestClient_GetExistingPostIDs(t *testing.T) {
	ctx, logger, env := testSetup(t)

	// Test cases
	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		posts map[int]domain.Post

		expextedErr error
	}{
		// Success cases
		"noPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: make(map[int]domain.Post),

			expextedErr: nil,
		},
		"onePostSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: map[int]domain.Post{
				post1.ID: post1,
			},

			expextedErr: nil,
		},
		"manyPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: map[int]domain.Post{
				post1.ID: post1,
				post2.ID: post2,
			},

			expextedErr: nil,
		},
		// Errors cases
		"authorNotFoundError": {
			setupFunc: func() {
				os.RemoveAll(fmt.Sprintf("%s/", env.MustGet(repository.BASE_PATH_ENV_KEY)))
			},
			teardownFunc: func() {},

			posts: nil,

			expextedErr: repository.ERR_AUTHOR_NOT_FOUND,
		},
		// TODO fail cases
		// TODO concurrency cases
	}

	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			// clearing test data after each test
			t.Cleanup(func() {
				clearTestData(t, env.MustGet(repository.BASE_PATH_ENV_KEY))
			})

			client, err := disk.New(logger, env, testProvider, testAuthor)
			if err != nil {
				t.Fatal(err)
			}

			test.setupFunc()
			defer test.teardownFunc()

			// saving posts to prepare test
			if test.posts != nil {
				err = client.SavePosts(ctx, &test.posts)
				if err != nil {
					t.Fatal(err)
				}
			}

			// getting existing post IDs
			gotPostIDs, gotErr := client.GetExistingPostIDs(ctx)
			assert.ErrorIs(t, gotErr, test.expextedErr)

			// validating got post IDs
			var expectedPostIDs []int = nil
			if test.posts != nil {
				expectedPostIDs = make([]int, 0, len(test.posts))
				for postID := range test.posts {
					expectedPostIDs = append(expectedPostIDs, postID)
				}
			}
			slices.Sort(expectedPostIDs)
			assert.Equal(t, expectedPostIDs, gotPostIDs)
		})
	}

}

// Depends on working New, SavePost
//
// Does not double GetPost/GetPosts and GetExistingPostIDs cases, tests only all posts managing
func TestClient_GetAllPosts(t *testing.T) {
	ctx, logger, env := testSetup(t)

	// Test cases
	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		posts map[int]domain.Post

		expextedErr error
	}{
		// Success cases
		"noPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: make(map[int]domain.Post, 0),

			expextedErr: nil,
		},
		"onePostSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: map[int]domain.Post{
				post1.ID: post1,
			},

			expextedErr: nil,
		},
		"manyPostsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			posts: map[int]domain.Post{
				post1.ID: post1,
				post2.ID: post2,
			},

			expextedErr: nil,
		},

		// Error cases
		"authorNotFoundError": {
			setupFunc: func() {
				os.RemoveAll(fmt.Sprintf("%s/", env.MustGet(repository.BASE_PATH_ENV_KEY)))
			},
			teardownFunc: func() {},

			posts: nil,

			expextedErr: repository.ERR_AUTHOR_NOT_FOUND,
		},
		// TODO fail cases
		// TODO concurrency cases
	}

	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			// clearing test data after each test
			t.Cleanup(func() {
				clearTestData(t, env.MustGet(repository.BASE_PATH_ENV_KEY))
			})

			client, err := disk.New(logger, env, testProvider, testAuthor)
			if err != nil {
				t.Fatal(err)
			}

			test.setupFunc()
			defer test.teardownFunc()

			// saving posts to prepare test
			if test.posts != nil {
				err = client.SavePosts(ctx, &test.posts)
				if err != nil {
					t.Fatal(err)
				}
			}

			// getting all posts
			gotPosts, gotErr := client.GetAllPosts(ctx)
			assert.ErrorIs(t, gotErr, test.expextedErr)
			assert.Equal(t, test.posts, gotPosts)
		})
	}
}

// SUPPORT FUNCTIONS

func authorPath() string {
	return fmt.Sprintf("%s/%s/%s", testBasePaths, testProvider, testAuthor)
}

func postPath(postID int) string {
	return fmt.Sprintf("%s/%d.json", authorPath(), postID)
}

func testSetup(t *testing.T) (context.Context, *slog.Logger, *appEnv.AppEnv) {
	envFile := ".env"
	content := []byte(fmt.Sprintf("%s=%s", repository.BASE_PATH_ENV_KEY, generateDirPath(t)))

	err := os.WriteFile(envFile, content, 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.Remove(envFile)
	})

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return t.Context(), logger, appEnv.New(logger)
}

func createMockPhoto(ID string) domain.Photo {
	return domain.Photo{
		Self:    createMockPicture(fmt.Sprintf("%s_%s", photoSelfPreffix, ID)),
		Preview: createMockPicture(fmt.Sprintf("%s_%s", previewPreffix, ID)),
	}
}

func createMockVideo(ID string) domain.Video {
	return domain.Video{
		Url:      fmt.Sprintf("%s_%s", videoSelfPreffix, ID),
		Filename: "",

		Title:       fmt.Sprintf("title_%s", ID),
		Description: fmt.Sprintf("description_%s", ID),
		Preview:     createMockPicture(fmt.Sprintf("%s_%s", previewPreffix, ID)),

		Content: []byte("AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAAAIZnJlZQAAAsxtZGF0AAACrQYF//+p3EXpvebZSLeWLNgg2SPu73gyNjQgLSBjb3JlIDE0OCByMjc0OCA5N2VhZWYyIC0gSC4yNjQvTVBFRy00IEFWQyBjb2RlYyAtIENvcHlsZWZ0IDIwMDMtMjAxNiAtIGh0dHA6Ly93d3cudmlkZW9sYW4ub3JnL3gyNjQuaHRtbCAtIG9wdGlvbnM6IGNhYmFjPTEgcmVmPTMgZGVibG9jaz0xOjA6MCBhbmFseXNlPTB4MzoweDExMyBtZT1oZXggc3VibWU9NyBwc3k9MSBwc3lfcmQ9MS4wMDowLjAwIG1peGVkX3JlZj0xIG1lX3JhbmdlPTE2IGNocm9tYV9tZT0xIHRyZWxsaXM9MSA4eDhkY3Q9MSBjcW09MCBkZWFkem9uZT0yMSwxMSBmYXN0X3Bza2lwPTEgY2hyb21hX3FwX29mZnNldD00IHRocmVhZHM9MSBsb29rYWhlYWRfdGhyZWFkcz0xIHNsaWNlZF90aHJlYWRzPTAgbnI9MCBkZWNpbWF0ZT0xIGludGVybGFjZWQ9MCBibHVyYXlfY29tcGF0PTAgY29uc3RyYWluZWRfaW50cmE9MCBiZnJhbWVzPTMgYl9weXJhbWlkPTIgYl9hZGFwdD0xIGJfYmlhcz0wIGRpcmVjdD0xIHdlaWdodGI9MSBvcGVuX2dvcD0wIHdlaWdodHA9MiBrZXlpbnQ9MjUwIGtleWludF9taW49MjUgc2NlbmVjdXQ9NDAgaW50cmFfcmVmcmVzaD0wIHJjX2xvb2thaGVhZD00MCByYz1jcmYgbWJ0cmVlPTEgY3JmPTIzLjAgcWNvbXA9MC42MCBxcG1pbj0wIHFwbWF4PTY5IHFwc3RlcD00IGlwX3JhdGlvPTEuNDAgYXE9MToxLjAwAIAAAAAPZYiEACv//vXb8yyubp//AAAC7W1vb3YAAABsbXZoZAAAAAAAAAAAAAAAAAAAA+gAAAAoAAEAAAEAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAIXdHJhawAAAFx0a2hkAAAAAwAAAAAAAAAAAAAAAQAAAAAAAAAoAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAQAAAAAAQAAAAEAAAAAAAJGVkdHMAAAAcZWxzdAAAAAAAAAABAAAAKAAAAAAAAQAAAAABj21kaWEAAAAgbWRoZAAAAAAAAAAAAAAAAAAAMgAAAAIAVcQAAAAAAC1oZGxyAAAAAAAAAAB2aWRlAAAAAAAAAAAAAAAAVmlkZW9IYW5kbGVyAAAAATptaW5mAAAAFHZtaGQAAAABAAAAAAAAAAAAAAAkZGluZgAAABxkcmVmAAAAAAAAAAEAAAAMdXJsIAAAAAEAAAD6c3RibAAAAJZzdHNkAAAAAAAAAAEAAACGYXZjMQAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAQABAASAAAAEgAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABj//wAAADBhdmNDAfQACv/hABdn9AAKkZsr02QAAAMABAAAAwDIPEiWWAEABmjr48RIRAAAABhzdHRzAAAAAAAAAAEAAAABAAACAAAAABxzdHNjAAAAAAAAAAEAAAABAAAAAQAAAAEAAAAUc3RzegAAAAAAAALEAAAAAQAAABRzdGNvAAAAAAAAAAEAAAAwAAAAYnVkdGEAAABabWV0YQAAAAAAAAAhaGRscgAAAAAAAAAAbWRpcmFwcGwAAAAAAAAAAAAAAAAtaWxzdAAAACWpdG9vAAAAHWRhdGEAAAABAAAAAExhdmY1Ny41Ni4xMDE="),
	}
}

func createMockPicture(preffix string) domain.Picture {
	img := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{1, 1}})
	img.Set(0, 0, color.Black)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		panic(err)
	}

	return domain.Picture{
		Url:      fmt.Sprintf("%s_%s", preffix, photoUrl),
		Filename: "",
		Content:  buf.Bytes(),
	}
}

func clearTestData(t *testing.T, dirPath string) {
	if err := os.RemoveAll(fmt.Sprintf("%s/", dirPath)); err != nil {
		t.Fatal(err)
	}
}

func generateDirPath(t *testing.T) string {
	path := fmt.Sprintf("%s/%s", t.TempDir(), testBasePaths)
	t.Log(path)
	return path
}
