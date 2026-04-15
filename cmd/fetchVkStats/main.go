package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"

	"arrai/config/appEnv"
	"arrai/internal/provider"
	"arrai/internal/provider/vk"
	"arrai/internal/repository"
	"arrai/internal/repository/disk"
)

func main() {
	ctx := context.Background()
	var err error

	// Create appEnv
	appEnvLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	appEnv := appEnv.New(appEnvLogger)
	debugMode := appEnv.GetBoolOrDefault("DEBUG_MODE", false)
	desktopMode := appEnv.GetBoolOrDefault("DESKTOP_MODE", false)

	// Create global logger
	loggerLevel := appEnv.GetIntOrDefault("LOGGER_LEVEL", 4)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.Level(loggerLevel),
	}))

	// Read provider type from user
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("Avaliable providers: %v\nDefault is 'vk'\nEnter provider type: ", provider.ProviderTypes)
	scanner.Scan()
	providerName := scanner.Text()
	if _, ok := provider.ProviderTypes[providerName]; !ok {
		providerName = "vk"
	}

	// Create VK client
	accessToken := appEnv.MustGet("VK_ACCESS_TOKEN")
	client := vk.NewClient(ctx, logger, appEnv, accessToken)

	// Read author ID from user
	fmt.Print("Enter author ID: ")
	scanner.Scan()
	authorID := scanner.Text()

	// Create repository
	// TODO: save to the real DB
	var repo repository.Repository
	if debugMode || desktopMode {
		repo, err = disk.New(ctx, logger, appEnv, providerName, authorID)
		if err != nil {
			logger.Error("failed to create repository",
				"error", err,
			)
			return
		}
	} else {
		// TODO DB
		logger.Error("Real DB is not implemented")
	}
	if repo == nil {
		logger.Error("failed to create repository",
			"error", "repository is nil",
		)
		return
	}

	// Get posts using API
	posts, err := client.GetPosts(authorID)
	if err != nil {
		logger.Error("failed to get posts",
			"error", err,
		)
		return
	} else {
		logger.Debug("Author's posts fetched",
			"author_id", authorID,
			"posts_count", len(*posts),
			"first_post_text", (*posts)[0].Text,
		)
	}

	// Save posts to repository
	if debugMode {
		defer repo.Clear()

		postID, err := repo.SavePost((*posts)[0])
		if err != nil {
			logger.Error("failed to save posts to repository",
				"error", err,
			)
			return
		}

		logger.Debug("First post saved to repository",
			"post_id", postID,
		)

		postIDs, err := repo.GetExistingPostIDs()
		if err != nil {
			logger.Error("failed to get existing post IDs from repository",
				"error", err,
			)
			return
		}
		logger.Debug("Existing post IDs fetched from repository",
			"post_ids", postIDs,
		)

		post, err := repo.GetPost(postIDs[0])
		if err != nil {
			logger.Error("failed to get first post from repository",
				"error", err,
			)
			return
		}
		logger.Debug("First post fetched from repository",
			"post_id", post.ID,
			"post_text", post.Text,
		)

		fmt.Print("Enter to clear debug repo: ")
		scanner.Scan()
	} else {
		repo.SavePosts(*posts)
	}
}
