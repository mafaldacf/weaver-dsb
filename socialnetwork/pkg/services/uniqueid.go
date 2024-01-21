package services

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"socialnetwork/pkg/model"

	"github.com/ServiceWeaver/weaver"
)

const CUSTOM_EPOCH int64 = 1514764800000

type UniqueIdService interface {
	UploadUniqueId(ctx context.Context, reqID int64, postType model.PostType) error
}

type uniqueIdService struct {
	weaver.Implements[UniqueIdService]
	composePostService 	weaver.Ref[ComposePostService]
	currentTimestamp 	int64
	counter 			int64
	machineId 			string
	mu 					sync.Mutex
}

func (u *uniqueIdService) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	u.machineId = getMachineID()
	logger.Info("unique id service running!", "machine_id", u.machineId)
	return nil
}

// From: https://gitlab.mpi-sws.org/cld/blueprint/systems/blueprint-dsb-socialnetwork/-/blob/main/input_v1/input_go/services/UniqueIdService.go
// From: https://gist.github.com/tsilvers/085c5f39430ced605d970094edf167ba
func getMachineID() string {
	interfaces, err := net.Interfaces()
    if err != nil {
        return "0"
    }

    for _, i := range interfaces {
        if i.Flags&net.FlagUp != 0 && !bytes.Equal(i.HardwareAddr, nil) {

            // Skip locally administered addresses
            if i.HardwareAddr[0]&2 == 2 {
                continue
            }

            var mac uint64
            for j, b := range i.HardwareAddr {
                if j >= 8 {
                    break
                }
                mac <<= 8
                mac += uint64(b)
            }

            return strconv.FormatUint(mac, 16)
        }
    }

    return "0"

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
	timestamp := time.Now().UnixMilli() - CUSTOM_EPOCH
	i, err := u.getCounter(timestamp)
	if err != nil {
		return err
	}
	timestampHex := strconv.FormatInt(timestamp, 16)
	
	if len(timestampHex) > 10 {
		timestampHex = timestampHex[:10]
	} else if len(timestampHex) < 10 {
		timestampHex = strings.Repeat("0", 10-len(timestampHex)) + timestampHex
	}
	counterHex := strconv.FormatInt(i, 16)
	if len(counterHex) > 3 {
		counterHex = counterHex[:3]
	} else if len(counterHex) < 3 {
		counterHex = strings.Repeat("0", 3-len(counterHex)) + counterHex
	}
	postIdStr := u.machineId + timestampHex + counterHex
	postId, err := strconv.ParseInt(postIdStr, 16, 64)
	if err != nil {
		return err
	}
	postId = postId & 0x7FFFFFFFFFFFFFFF
	return u.composePostService.Get().UploadUniqueId(ctx, reqID, postId, postType)
}
