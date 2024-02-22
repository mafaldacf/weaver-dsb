package services

import (
	"context"
	"fmt"
	"socialnetwork/pkg/model"

	"github.com/ServiceWeaver/weaver"
)

type MediaService interface {
	UploadMedia(ctx context.Context, reqID int64, mediaTypes []string, mediaIDs []int64, edited bool) error
}

type mediaService struct {
	weaver.Implements[MediaService]
	weaver.WithConfig[mediaServiceOptions]
	composePostService   weaver.Ref[ComposePostService]
}

type mediaServiceOptions struct {
	Region    string `toml:"region"`
}

func (m *mediaService) Init(ctx context.Context) error {
	logger := m.Logger(ctx)
	logger.Info("media service running!", "region", m.Config().Region)
	return nil
}

func (m *mediaService) UploadMedia(ctx context.Context, reqID int64, mediaTypes []string, mediaIDs []int64, edited bool) error {
	logger := m.Logger(ctx)
	logger.Debug("entering UploadMedia", "req_id", reqID, "media_types", mediaTypes, "mediaIDs", mediaIDs)
	if len(mediaTypes) != len(mediaIDs) {
		errMsg := "the lengths of media_id list and media_type list are not equal"
		logger.Error(errMsg, "num_media_types", len(mediaTypes), "num_media_ids", len(mediaIDs))
		return fmt.Errorf(errMsg)
	}
	var medias []model.Media
	for i := range mediaIDs {
		medias = append(medias, model.Media {
			MediaID: mediaIDs[i],
			MediaType: mediaTypes[i],
		})
	}
	if edited {
		return m.composePostService.Get().UploadEditedMedia(ctx, reqID, medias)
	}
	return m.composePostService.Get().UploadMedia(ctx, reqID, medias)
}
