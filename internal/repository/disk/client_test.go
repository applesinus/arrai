package disk_test

import (
	"arrai/config/appEnv"
	"arrai/internal/domain"
	"arrai/internal/repository"
	"arrai/internal/repository/disk"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testBasePath = "/temp/testDiskRepo"
	testProvider = "provider"
	testAuthor   = "author"

	photoSelfPreffix    = "big"
	photoPreviewPreffix = "small"
	photoUrl            = "url"
	photoFilename       = "filename"
)

func authorPath() string {
	return fmt.Sprintf("%s/%s/%s", testBasePath, testProvider, testAuthor)
}

func postPath(postID int) string {
	return fmt.Sprintf("%s/%d.json", authorPath(), postID)
}

func testSetup(t *testing.T) {
	envFile := ".env"
	content := []byte(fmt.Sprintf("%s=%s", repository.BASE_PATH_ENV_KEY, testBasePath))

	err := os.WriteFile(envFile, content, 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.Remove(envFile)
	})
}

func createMockPhoto(t *testing.T, preffix string) domain.Photo {
	img := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{1, 1}})
	img.Set(0, 0, color.Black)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		t.Fatal(err)
	}

	return domain.Photo{
		Url:      fmt.Sprintf("%s_%s", preffix, photoUrl),
		Filename: fmt.Sprintf("%s_%s", preffix, photoFilename),
		Content:  buf.Bytes(),
	}
}

func createMockPhotoWithPreview(t *testing.T, ID string) domain.PhotoWithPreview {
	return domain.PhotoWithPreview{
		Self:    createMockPhoto(t, fmt.Sprintf("%s_%s", photoSelfPreffix, ID)),
		Preview: createMockPhoto(t, fmt.Sprintf("%s_%s", photoPreviewPreffix, ID)),
	}
}

func TestNew(t *testing.T) {
	testSetup(t)

	// Creating test values
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	env := appEnv.New(logger)
	basePath := testBasePath

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
				err := os.MkdirAll(fmt.Sprintf("%s/%s", testBasePath, testProvider), 0755)
				if err != nil {
					t.Fatal(err)
				}
			},
			teardownFunc: func() {
				err := os.RemoveAll(fmt.Sprintf("%s/%s", testBasePath, testProvider))
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
				err := os.MkdirAll(testBasePath+fmt.Sprintf("/%s/%s", testProvider, testAuthor), 0755)
				if err != nil {
					t.Fatal(err)
				}
			},
			teardownFunc: func() {
				err := os.RemoveAll(fmt.Sprintf("%s/%s/%s", testBasePath, testProvider, testAuthor))
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
				t.Setenv(repository.BASE_PATH_ENV_KEY, testBasePath)
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

func TestSavePost(t *testing.T) {
	testSetup(t)

	// Creating test values
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	env := appEnv.New(logger)
	ctx := t.Context()

	ID := 1
	ownerID := 2
	creationTime := time.Now().Truncate(0)
	views := 3
	reactions := 4
	reposts := 5
	text := "text"
	photo1 := createMockPhotoWithPreview(t, "1")
	photo2 := createMockPhotoWithPreview(t, "2")
	comment1 := domain.Comment{
		ID:        6,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 7,
		Text:      "text",
		Photos:    []domain.PhotoWithPreview{},
		Replies:   []domain.Comment{},
	}
	comment2 := domain.Comment{
		ID:        8,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 9,
		Text:      "text",
		Photos:    []domain.PhotoWithPreview{},
		Replies:   []domain.Comment{},
	}
	commentWithOnePhoto := domain.Comment{
		ID:        10,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 11,
		Text:      "text",
		Photos: []domain.PhotoWithPreview{
			photo1,
		},
		Replies: []domain.Comment{},
	}
	commentWithManyPhotos := domain.Comment{
		ID:        12,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 13,
		Text:      "text",
		Photos: []domain.PhotoWithPreview{
			photo1,
			photo2,
		},
		Replies: []domain.Comment{},
	}
	thread := domain.Comment{
		ID:        14,
		CreatedAt: time.Now().Truncate(0),
		User:      "user",
		IsAuthor:  true,
		Reactions: 15,
		Text:      "text",
		Photos:    []domain.PhotoWithPreview{},
		Replies:   []domain.Comment{comment1, comment2},
	}

	// Test cases
	testsTable := map[string]struct {
		setupFunc    func()
		teardownFunc func()

		post domain.Post

		expectedInt int
		expextedErr error
	}{
		// Success cases
		"postWithTextSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.PhotoWithPreview{},
				Comments:  []domain.Comment{},
			},

			expectedInt: ID,
			expextedErr: nil,
		},
		"postWithOnePhotoSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos: []domain.PhotoWithPreview{
					photo1,
				},
				Comments: []domain.Comment{},
			},

			expectedInt: ID,
			expextedErr: nil,
		},
		"postWithManyPhotosSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos: []domain.PhotoWithPreview{
					photo1,
					photo2,
				},
				Comments: []domain.Comment{},
			},

			expectedInt: ID,
			expextedErr: nil,
		},
		"postWithOneCommentSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.PhotoWithPreview{},
				Comments: []domain.Comment{
					comment1,
				},
			},

			expectedInt: ID,
			expextedErr: nil,
		},
		"postWithManyCommentsSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.PhotoWithPreview{},
				Comments: []domain.Comment{
					comment1,
					comment2,
				},
			},

			expectedInt: ID,
			expextedErr: nil,
		},
		"postWithPhotoInComment": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.PhotoWithPreview{},
				Comments: []domain.Comment{
					commentWithOnePhoto,
				},
			},

			expectedInt: ID,
			expextedErr: nil,
		},
		"postWithNamyPhotosInComment": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.PhotoWithPreview{},
				Comments: []domain.Comment{
					commentWithManyPhotos,
				},
			},

			expectedInt: ID,
			expextedErr: nil,
		},
		"postWithCommentsThreadSuccess": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        ID,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.PhotoWithPreview{},
				Comments: []domain.Comment{
					thread,
				},
			},

			expectedInt: ID,
			expextedErr: nil,
		},

		// Errors cases
		"noPostID": {
			setupFunc:    func() {},
			teardownFunc: func() {},

			post: domain.Post{
				ID:        -1,
				OwnerID:   ownerID,
				CreatedAt: creationTime,
				Views:     views,
				Reactions: reactions,
				Reposts:   reposts,
				Text:      text,
				Photos:    []domain.PhotoWithPreview{},
				Comments:  []domain.Comment{},
			},

			expectedInt: -1,
			expextedErr: domain.ERROR_NO_POST_ID,
		},
	}

	// Running tests
	for testName, test := range testsTable {
		t.Run(testName, func(t *testing.T) {
			client, err := disk.New(logger, env, testProvider, testAuthor)
			if err != nil {
				t.Fatal(err)
			}

			test.setupFunc()
			defer test.teardownFunc()

			gotInt, gotErr := client.SavePost(ctx, test.post)
			assert.ErrorIs(t, gotErr, test.expextedErr)
			assert.Equal(t, test.expectedInt, gotInt)

			if gotErr == nil {
				fullPath := fmt.Sprintf("%s/%s/%s/%d.json", testBasePath, testProvider, testAuthor, ID)

				_, err = os.Stat(fullPath)
				if os.IsNotExist(err) {
					t.Errorf("File with ID %d not created", ID)
				} else {
					file, err := os.OpenFile(fullPath, os.O_RDONLY, 0644)
					if err != nil {
						t.Errorf("File with ID %d could not be opened", ID)
					} else {
						func() {
							defer file.Close()

							var post domain.Post
							err = json.NewDecoder(file).Decode(&post)
							if err != nil {
								t.Errorf("File with ID %d could not be decoded", ID)
							} else {
								assert.Equal(t, test.post, post)
							}
						}()
					}
				}
			}

			// clearing test data
			err = os.RemoveAll(fmt.Sprintf("%s/", testBasePath))
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
