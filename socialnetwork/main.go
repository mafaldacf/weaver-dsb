//go:generate weaver generate . ./services ./services/model

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"socialnetwork/services"
	"socialnetwork/services/model"
	"time"

	"github.com/ServiceWeaver/weaver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	if err := weaver.Run(context.Background(), serve); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	weaver.Implements[weaver.Main]
	composePost weaver.Ref[services.ComposePost]
	_           weaver.Ref[services.WriteHomeTimeline]
	lis         weaver.Listener `weaver:"wrk2"`
}

func serve(ctx context.Context, s *server) error {
	mux := http.NewServeMux()
	mux.Handle("/composepost", instrument("composepost", s.composePostHandler, http.MethodGet))
	var handler http.Handler = mux
	s.Logger(ctx).Info("wrk2-api available", "addr", s.lis)
	return http.Serve(s.lis, handler)
}

func (s *server) composePostHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering rootHandler")

	trace.SpanFromContext(r.Context()).AddEvent("handling http requesdt",
		trace.WithAttributes(
			attribute.String("content", "hello there"),
		))

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	reqID := rand.Int63()
	creator := model.Creator {
		UserID: 0,
		Username: "user_0",
	}

	err := s.composePost.Get().UploadCreator(ctx, reqID, creator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := "success! :)\n"
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
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
