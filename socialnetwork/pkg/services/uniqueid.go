package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/utils"

	"github.com/ServiceWeaver/weaver"
)

type UniqueIdService interface {
	UploadUniqueId(ctx context.Context, reqID int64, postType model.PostType) error
}

type uniqueIdOptions struct {
	Region    string `toml:"region"`
}

type uniqueIdService struct {
	weaver.Implements[UniqueIdService]
	weaver.WithConfig[uniqueIdOptions]
	composePostService weaver.Ref[ComposePostService]
	currentTimestamp   int64
	counter            int64
	machineID          string
	mu                 sync.Mutex
}

func (u *uniqueIdService) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	u.machineID = utils.GetMachineID()
	u.currentTimestamp = -1
	u.counter = 0
	logger.Info("unique id service running!", "machine_id", u.machineID, "region", u.Config().Region)
	return nil
}

func (u *uniqueIdService) getCounter(timestamp int64) (int64, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.currentTimestamp > timestamp {
		return 0, fmt.Errorf("timestamps are not incremental")
	}
	if u.currentTimestamp == timestamp {
		counter := u.counter
		u.counter += 1
		return counter, nil
	} else {
		u.currentTimestamp = timestamp
		u.counter = 1
		return u.counter, nil
	}

}

func (u *uniqueIdService) UploadUniqueId(ctx context.Context, reqID int64, postType model.PostType) error {
	logger := u.Logger(ctx)
	logger.Debug("entering UploadUniqueId", "req_id", reqID, "post_type", postType)

	timestamp := time.Now().UnixMilli() - utils.CUSTOM_EPOCH
	counter, err := u.getCounter(timestamp)
	if err != nil {
		logger.Error("error getting counter", "msg", err.Error())
		return err
	}
	id, err := utils.GenUniqueID(u.machineID, timestamp, counter)
	if err != nil {
		return err
	}
	return u.composePostService.Get().UploadUniqueId(ctx, reqID, id, postType)
}
