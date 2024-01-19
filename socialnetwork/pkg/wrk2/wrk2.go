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
	lis         		weaver.Listener `weaver:"wrk2"`
}

func Serve(ctx context.Context, s *server) error {
	mux := http.NewServeMux()
	mux.Handle("/composepost", instrument("composepost", s.composePostHandler, http.MethodGet))
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
	medias := []model.Media {
		{
			MediaID: 0,
			MediaType: "png",
		},
		{
			MediaID: 1,
			MediaType: "png",
		},
	}
	postID := rand.Int63()
	postType := model.POST_TYPE_POST
	creator := model.Creator {
		UserID: 0,
		Username: "user_0",
	}
	urls := []model.URL {
		{
			ShortenURL: "shortened_url_0",
			ExpandedURL: "expanded_url_0",
		},
		{
			ShortenURL: "shortened_url_1",
			ExpandedURL: "expanded_url_1",
		},
	}
	userMentions := []model.UserMention {
		{
			UserID: 1,
			Username: "user_1",
		},
		{
			UserID: 1,
			Username: "user_1",
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
