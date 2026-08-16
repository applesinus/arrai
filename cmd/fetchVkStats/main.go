package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"arrai/config/appEnv"
	"arrai/internal/provider"
	"arrai/internal/provider/vk"
	"arrai/internal/repository"
	"arrai/internal/repository/disk"
)

func main() {
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

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
	client := vk.NewClient(wg, logger, appEnv, accessToken)

	// Read author ID from user
	fmt.Print("Enter author ID: ")
	scanner.Scan()
	authorID := scanner.Text()

	// Create repository
	// TODO: save to the real DB
	var repo repository.Repository
	if debugMode || desktopMode {
		repo, err = disk.New(logger, appEnv, providerName, authorID)
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

	// Timer setup
	startTime := time.Now()
	defer func() {
		fmt.Printf("\n==========\n\nExecution time: %v", time.Since(startTime))
	}()
	go timer(ctx, logger, wg, startTime)

	// Get posts using API
	posts, err := client.GetPosts(ctx, authorID)
	if err != nil {
		logger.Error("failed to get posts",
			"error", err,
		)
		return
	} else {
		logger.Info("Posts fetched from VK API",
			"author_id", authorID,
			"posts_count", len(*posts),
		)

		logger.Debug("First post",
			"text", (*posts)[0].Text,
		)
	}

	// Save posts to repository
	if debugMode {
		defer repo.Clear(ctx)

		err := repo.SavePost(ctx, (*posts)[0])
		if err != nil {
			logger.Error("failed to save posts to repository",
				"error", err,
			)
			return
		}

		logger.Debug("First post saved to repository",
			"post_id", (*posts)[0].ID,
		)

		postIDs, err := repo.GetExistingPostIDs(ctx)
		if err != nil {
			logger.Error("failed to get existing post IDs from repository",
				"error", err,
			)
			return
		}
		logger.Debug("Existing post IDs fetched from repository",
			"post_ids", postIDs,
		)

		post, err := repo.GetPost(ctx, postIDs[0])
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
		logger.Info("Saving posts to repository")
		repo.SavePosts(ctx, *posts)
		logger.Info("Posts are saved to repository")
	}
}

func timer(ctx context.Context, logger *slog.Logger, wg *sync.WaitGroup, timeStart time.Time) {
	wg.Add(1)
	defer func() {
		wg.Done()
		logger.Info("Timer stopped")
	}()

	logger.Info("Timer started")

	timer := time.NewTimer(1 * time.Minute)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			minutesPassed := int(time.Since(timeStart).Minutes())
			logger.Info("WIP",
				"minutes_passed", minutesPassed,
			)
			timer.Reset(time.Minute * time.Duration(minutesPassed%10+1))
		}
	}
}
