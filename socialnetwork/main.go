//go:generate weaver generate . ./services

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"socialnetwork/services"

	"github.com/ServiceWeaver/weaver"
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

	err := s.composePost.Get().ComposeAndUpload(ctx, "google")
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
