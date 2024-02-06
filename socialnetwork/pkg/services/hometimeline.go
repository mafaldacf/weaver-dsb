package services

import (
	"context"
	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"
	"strconv"

	"github.com/ServiceWeaver/weaver"

	"github.com/redis/go-redis/v9"
)

type HomeTimelineService interface {
	ReadHomeTimeline(ctx context.Context, reqID int64, userID int64, start int64, stop int64) ([]model.Post, error)
}

type homeTimelineService struct {
	weaver.Implements[HomeTimelineService]
	weaver.WithConfig[homeTimelineServiceOptions]
	postStorageService   weaver.Ref[PostStorageService]
	redisClient   		 *redis.Client
}

type homeTimelineServiceOptions struct {
	RedisAddr        string   `toml:"redis_address"`
	RedisPort        int      `toml:"redis_port"`
}

func (h *homeTimelineService) Init(ctx context.Context) error {
	logger := h.Logger(ctx)
	h.redisClient = storage.RedisClient(h.Config().RedisAddr, h.Config().RedisPort)
	logger.Info("home timeline service running!", "rabbitmq_addr", h.Config().RedisAddr, "rabbitmq_port", h.Config().RedisPort)
	return nil
}

// readCachedTimeline is an helper function for reading timeline from redis with the same behavior as in the user timeline service
func (h *homeTimelineService) readCachedTimeline(ctx context.Context, userID int64, start int64, stop int64) ([]int64, error) {
	logger := h. Logger(ctx)
	
	userIDStr := strconv.FormatInt(userID, 10)
	result, err := h. redisClient.ZRevRange(ctx, userIDStr, start, stop-1).Result()
	if err != nil {
		logger.Error("error reading home timeline from redis")
		return nil, err
	}
	
	var postIDs []int64
	for _, result := range result {
		id, err := strconv.ParseInt(result, 10, 64)
		if err != nil {
			logger.Error("error parsing post id from redis result")
			return nil, err
		}
		postIDs = append(postIDs, id)
	}
	return postIDs, nil
}

func (h *homeTimelineService) ReadHomeTimeline(ctx context.Context, reqID int64, userID int64, start int64, stop int64) ([]model.Post, error) {
	logger := h.Logger(ctx)
	logger.Debug("entering ReadHomeTimeline", "req_id", reqID, "user_id", userID, "start", start, "stop", stop)
	if stop <= start || start < 0 {
		return []model.Post{}, nil
	}

	postIDs, err := h.readCachedTimeline(ctx, userID, start, stop)
	if err != nil {
		return nil, err
	}
	return h.postStorageService.Get().ReadPosts(ctx, reqID, postIDs)
}
