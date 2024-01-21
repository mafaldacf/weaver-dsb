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
	UploadText(ctx context.Context, reqID int64, text string) error
}

type textService struct {
	weaver.Implements[TextService]
	composePostService   weaver.Ref[ComposePostService]
	urlShortenService    weaver.Ref[UrlShortenService]
	userMentionService   weaver.Ref[UserMentionService]
}

func (t *textService) Init(ctx context.Context) error {
	logger := t.Logger(ctx)
	logger.Info("text service running!")
	return nil
}

func (t *textService) UploadText(ctx context.Context, reqID int64, text string) error {
	r := regexp.MustCompile(`@[a-zA-Z0-9-_]+`)
	matches := r.FindAllString(text, -1)
	var usernames []string
	for _, m := range matches {
		usernames = append(usernames, m[1:])
	}
	url_re := regexp.MustCompile(`(http://|https://)([a-zA-Z0-9_!~*'().&=+$%-]+)`)
	url_strings := url_re.FindAllString(text, -1)

	var errs [2]error
	var urls []model.URL
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = t.urlShortenService.Get().UploadUrls(ctx, reqID, url_strings)
	}()
	go func() {
		defer wg.Done()
		errs[1] = t.userMentionService.Get().UploadUserMentions(ctx, reqID, usernames)
	}()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	updatedText := text
	if len(urls) != 0 {
		for idx, url_string := range url_strings {
			updatedText = strings.ReplaceAll(updatedText, url_string, urls[idx].ShortenedUrl)
		}
	}

	return t.composePostService.Get().UploadText(ctx, reqID, updatedText)
}
