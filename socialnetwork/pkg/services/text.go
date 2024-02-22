package services

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"socialnetwork/pkg/model"

	"github.com/ServiceWeaver/weaver"
)

type TextService interface {
	UploadText(ctx context.Context, reqID int64, text string, edited bool) error
}

type textServiceOptions struct {
	Region string `toml:"region"`
}

type textService struct {
	weaver.Implements[TextService]
	weaver.WithConfig[textServiceOptions]
	composePostService weaver.Ref[ComposePostService]
	urlShortenService  weaver.Ref[UrlShortenService]
	userMentionService weaver.Ref[UserMentionService]
}

func (t *textService) Init(ctx context.Context) error {
	logger := t.Logger(ctx)
	logger.Info("text service running!", "region", t.Config().Region)
	return nil
}

func (t *textService) UploadText(ctx context.Context, reqID int64, text string, edited bool) error {
	logger := t.Logger(ctx)
	logger.Debug("entering UploadText", "req_id", reqID, "text", text)

	r := regexp.MustCompile(`@[a-zA-Z0-9-_]+`)
	matches := r.FindAllString(text, -1)
	var usernames []string
	for _, m := range matches {
		usernames = append(usernames, m[1:])
	}
	logger.Debug("usernames mentioned", "u", usernames)
	url_re := regexp.MustCompile(`(http://|https://)([a-zA-Z0-9_!~*'().&=+$%-]+)`)
	url_strings := url_re.FindAllString(text, -1)

	var shortenUrlErr, userMentionErr, uploadTextErr error
	var shortenUrlWg, userMentionWg, uploadTextWg sync.WaitGroup
	var urls []model.URL

	// -- url shorten service rpc
	shortenUrlWg.Add(1)
	go func() {
		defer shortenUrlWg.Done()
		shortenUrlErr = t.urlShortenService.Get().UploadUrls(ctx, reqID, url_strings)
	}()
	// --

	// -- user mention service rpc
	userMentionWg.Add(1)
	go func() {
		defer userMentionWg.Done()
		userMentionErr = t.userMentionService.Get().UploadUserMentions(ctx, reqID, usernames)
	}()
	// --

	shortenUrlWg.Wait()
	if shortenUrlErr != nil {
		logger.Error("error uploading urls to url shorten service", "msg", shortenUrlErr.Error())
		return shortenUrlErr
	}

	updatedText := text
	if len(urls) != 0 {
		for idx, url_string := range url_strings {
			updatedText = strings.ReplaceAll(updatedText, url_string, urls[idx].ShortenedUrl)
		}
	}

	// -- compose post service rpc
	uploadTextWg.Add(1)
	go func() {
		defer uploadTextWg.Done()
		if edited {
			uploadTextErr = t.composePostService.Get().UploadEditedText(ctx, reqID, updatedText)
		} else {
			uploadTextErr = t.composePostService.Get().UploadText(ctx, reqID, updatedText)
		}
	}()
	// --

	userMentionWg.Wait()
	if userMentionErr != nil {
		logger.Error("error uploading user mentions to user mention service", "msg", userMentionErr.Error())
		return userMentionErr
	}
	uploadTextWg.Wait()
	if uploadTextErr != nil {
		logger.Error("error uploading text to compose post service", "msg", uploadTextErr.Error())
		return uploadTextErr
	}

	return uploadTextErr
}
