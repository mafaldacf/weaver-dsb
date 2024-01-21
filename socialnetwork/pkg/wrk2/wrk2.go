package wrk2

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/services"

	"github.com/ServiceWeaver/weaver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type server struct {
	weaver.Implements[weaver.Main]
	composePostService 	weaver.Ref[services.ComposePostService]
	homeTimelineService weaver.Ref[services.HomeTimelineService]
	userTimelineService weaver.Ref[services.UserTimelineService]
	textService 		weaver.Ref[services.TextService]
	mediaService 		weaver.Ref[services.MediaService]
	uniqueIdService 	weaver.Ref[services.UniqueIdService]
	userService 		weaver.Ref[services.UserService]
	lis                	weaver.Listener `weaver:"wrk2"`
}

func Serve(ctx context.Context, s *server) error {
	mux := http.NewServeMux()
	mux.Handle("/composepost", instrument("composepost", s.composePostHandler, http.MethodGet))
	mux.Handle("/composepost2", instrument("composepost2", s.composePostHandler2, http.MethodGet))
	var handler http.Handler = mux
	s.Logger(ctx).Info("wrk2-api available", "addr", s.lis)
	return http.Serve(s.lis, handler)
}

func instrument(label string, fn func(http.ResponseWriter, *http.Request), methods ...string) http.Handler {
	allowed := map[string]struct{}{}
	for _, method := range methods {
		allowed[method] = struct{}{}
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, ok := allowed[r.Method]; len(allowed) > 0 && !ok {
			msg := fmt.Sprintf("method %q not allowed", r.Method)
			http.Error(w, msg, http.StatusMethodNotAllowed)
			return
		}
		fn(w, r)
	}
	return weaver.InstrumentHandlerFunc(label, handler)
}

func (s *server) composePostHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering rootHandler")

	trace.SpanFromContext(r.Context()).AddEvent("handling http requesdt",
		trace.WithAttributes(
			attribute.String("content", "hello there"),
		))

	text := "HelloWorld"
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	reqID := rand.Int63()
	medias := []model.Media{
		{
			MediaID:   0,
			MediaType: "png",
		},
		{
			MediaID:   1,
			MediaType: "png",
		},
	}
	postID := rand.Int63()
	postType := model.POST_TYPE_POST
	creator := model.Creator{
		UserID:   0,
		Username: "user_0",
	}
	urls := []model.URL{
		{
			ShortenedUrl: "shortened_url_0",
			ExpandedUrl:  "expanded_url_0",
		},
		{
			ShortenedUrl: "shortened_url_1",
			ExpandedUrl:  "expanded_url_1",
		},
	}
	userMentions := []model.UserMention{
		{
			UserID:   1,
			Username: "user_1",
		},
		{
			UserID:   2,
			Username: "user_2",
		},
	}

	var wg sync.WaitGroup
	wg.Add(6)
	var errs [6]error
	go func() {
		errs[0] = s.composePostService.Get().UploadCreator(ctx, reqID, creator)
		defer wg.Done()
	}()
	go func() {
		errs[1] = s.composePostService.Get().UploadText(ctx, reqID, text)
		defer wg.Done()
	}()
	go func() {
		errs[2] = s.composePostService.Get().UploadMedia(ctx, reqID, medias)
		defer wg.Done()
	}()
	go func() {
		errs[3] = s.composePostService.Get().UploadUniqueId(ctx, reqID, postID, postType)
		defer wg.Done()
	}()
	go func() {
		errs[4] = s.composePostService.Get().UploadUrls(ctx, reqID, urls)
		defer wg.Done()
	}()
	go func() {
		errs[5] = s.composePostService.Get().UploadUserMentions(ctx, reqID, userMentions)
		defer wg.Done()
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	response := "success! :)\n"
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
}


func (s *server) composePostHandler2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering rootHandler")

	trace.SpanFromContext(r.Context()).AddEvent("handling http requesdt",
		trace.WithAttributes(
			attribute.String("content", "hello there"),
		))

	text := "HelloWorld"
	var userID int64 = 0
	username := "user_0"
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	reqID := rand.Int63()
	mediaTypes := []string{"png", "png"}
	mediaIDs := []int64{0, 1}
	postType := model.POST_TYPE_POST

	var wg sync.WaitGroup
	wg.Add(4)
	var errs [4]error
	go func() {
		defer wg.Done()
		errs[0] = s.textService.Get().UploadText(ctx, reqID, text)
		logger.Debug("upload text done!")
	}()
	go func() {
		defer wg.Done()
		errs[1] = s.mediaService.Get().UploadMedia(ctx, reqID, mediaTypes, mediaIDs)
		logger.Debug("upload media done!")
	}()
	go func() {
		defer wg.Done()
		errs[2] = s.uniqueIdService.Get().UploadUniqueId(ctx, reqID, postType)
		logger.Debug("upload unique id done!")
	}()
	go func() {
		defer wg.Done()
		errs[3] = s.userService.Get().UploadCreatorWithUserId(ctx, reqID, userID, username)
		logger.Debug("upload creator with user id done!")
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	response := "success! :)\n"
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
}
