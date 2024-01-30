package wrk2

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/services"

	"github.com/ServiceWeaver/weaver"
)

type server struct {
	weaver.Implements[weaver.Main]
	homeTimelineService weaver.Ref[services.HomeTimelineService]
	userTimelineService weaver.Ref[services.UserTimelineService]
	textService 		weaver.Ref[services.TextService]
	mediaService 		weaver.Ref[services.MediaService]
	uniqueIdService 	weaver.Ref[services.UniqueIdService]
	userService 		weaver.Ref[services.UserService]
	socialGraphService 	weaver.Ref[services.SocialGraphService]
	lis                	weaver.Listener `weaver:"wrk2"`
}

func Serve(ctx context.Context, s *server) error {
	mux := http.NewServeMux()

	// declare api endpoints
	mux.Handle("/wrk2-api/user/register", instrument("user/register", s.registerHandler, http.MethodGet, http.MethodPost))
	mux.Handle("/wrk2-api/user/follow", instrument("user/follow", s.followHandler, http.MethodGet, http.MethodPost))
	mux.Handle("/wrk2-api/user/unfollow", instrument("user/unfollow", s.unfollowHandler, http.MethodGet, http.MethodPost))
	mux.Handle("/wrk2-api/user/login", instrument("user/login", s.loginHandler, http.MethodGet, http.MethodPost))
	mux.Handle("/wrk2-api/post/compose", instrument("post/compose", s.composePostHandler, http.MethodGet, http.MethodPost))
	mux.Handle("/wrk2-api/home-timeline/read", instrument("home/timeline", s.readHomeTimelineHandler, http.MethodGet, http.MethodPost))
	mux.Handle("/wrk2-api/user-timeline/read", instrument("user/timeline", s.readUserTimelineHandler, http.MethodGet, http.MethodPost))

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

type registerParams struct {
	reqID 		int64
	firstName 	string
	lastName 	string
	username 	string
	password 	string
	userID 		int64
}

func genRegisterParams() registerParams {
	params := registerParams {
		reqID: 		rand.New(rand.NewSource(time.Now().UnixNano())).Int63(),
		firstName: 	"firstname_0",
		lastName: 	"lastname_0",
		username: 	"user_0",
		password: 	"password_0",
		userID: 	0,
	}
	return params
}

func (s *server) registerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering wkr2-api/user/register")

    if err := r.ParseForm(); err != nil {
        http.Error(w, "error: " + err.Error(), http.StatusBadRequest)
        return
    }
	var err error
	params := genRegisterParams()
    params.username = r.Form.Get("username")
    params.firstName = r.Form.Get("first_name")
    params.lastName = r.Form.Get("last_name")
    params.password = r.Form.Get("password")
	params.userID = -1
	userIDstr := r.Form.Get("user_id")
	
    if err != nil {
        http.Error(w, "invalid user id", http.StatusBadRequest)
        return
    }
    if params.username == "" {
        http.Error(w, "must provide a valid username", http.StatusBadRequest)
		return
    }
	if params.firstName == "" {
        http.Error(w, "must provide a valid first name", http.StatusBadRequest)
		return
    }
	if params.lastName == "" {
        http.Error(w, "must provide a valid last name", http.StatusBadRequest)
		return
    }
	if userIDstr != "" {
		params.userID, err = strconv.ParseInt(userIDstr, 10, 64)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}
	}

	if params.userID == -1 {
		err = s.userService.Get().RegisterUser(ctx, params.reqID, params.firstName, params.lastName, params.username, params.password)
	} else {
		err = s.userService.Get().RegisterUserWithId(ctx, params.reqID, params.firstName, params.lastName, params.username, params.password, params.userID)
	}
	if err != nil {
		http.Error(w, "error: " + err.Error(), http.StatusInternalServerError)
		return
	}

	response := fmt.Sprintf("success! registered user %s (id=%d)\n", params.username, params.userID)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
}

type followParams struct {
	reqID 			int64
	userID 			int64
	followee_id 	int64
	username 		string
	followee_name 	string
}

func genFollowParams() followParams {
	params := followParams {
		reqID: 			rand.New(rand.NewSource(time.Now().UnixNano())).Int63(),
		userID: 		0,
		followee_id: 	1,
		username: 		"user_0",
		followee_name: 	"user_1",
	}
	return params
}

func (s *server) followHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering wkr2-api/user/follow")
	
	var err error
	params := genFollowParams()
	if params.userID != -1 && params.followee_id != -1 { 
		err = s.socialGraphService.Get().Follow(ctx, params.reqID, params.userID, params.followee_id)
	} else if params.username != "" && params.followee_name != "" {
		err = s.socialGraphService.Get().FollowWithUsername(ctx, params.reqID, params.username, params.followee_name)
	} else {
		err = fmt.Errorf("invalid arguments")
	}

	if err != nil {
		http.Error(w, "error: " + err.Error(), http.StatusBadRequest)
		return
	}
	response := fmt.Sprintf("success! user %s (id=%d) followed user %s (id=%d)\n", params.username, params.userID, params.followee_name, params.followee_id)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
}

func (s *server) unfollowHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering wkr2-api/user/unfollow")

	//TODO!!!

	response := "success! :)\n"
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
}

type LoginParams struct {
	reqID 		int64
	username 	string
	password 	string
}

func getLoginParams() LoginParams {
	params := LoginParams {
		reqID: 		rand.New(rand.NewSource(time.Now().UnixNano())).Int63(),
		username: 	"user_0",
		password: 	"password_0",
	}
	return params
}

func (s *server) loginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering wkr2-api/user/login")

	params := getLoginParams()
	token, err := s.userService.Get().Login(ctx, params.reqID, params.username, params.password)
	if err != nil {
		http.Error(w, "error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	
	response := fmt.Sprintf("success! user %s logged in with token %s\n", params.username, token)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
}

type composePostParams struct {
	text 		string
	userID 	int64
	username 	string
	reqID 		int64
	mediaTypes 	[]string
	mediaIDs 	[]int64
	postType 	model.PostType
}

func genComposePostParams() composePostParams {
	params := composePostParams {
		text: 		"HelloWorld",
		userID: 	0,
		username: 	"user_0",
		reqID: 		rand.New(rand.NewSource(time.Now().UnixNano())).Int63(),
		mediaTypes: []string{"png", "png"},
		mediaIDs: 	[]int64{0, 1},
		postType: 	model.POST_TYPE_POST,
	}
	return params
}

func (s *server) composePostHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering wkr2-api/post/compose")

	params := genComposePostParams()
	var wg sync.WaitGroup
	wg.Add(4)
	var errs [4]error
	go func() {
		defer wg.Done()
		errs[0] = s.textService.Get().UploadText(ctx, params.reqID, params.text)
		logger.Debug("upload text done!")
	}()
	go func() {
		defer wg.Done()
		errs[1] = s.mediaService.Get().UploadMedia(ctx, params.reqID, params.mediaTypes, params.mediaIDs)
		logger.Debug("upload media done!")
	}()
	go func() {
		defer wg.Done()
		errs[2] = s.uniqueIdService.Get().UploadUniqueId(ctx, params.reqID, params.postType)
		logger.Debug("upload unique id done!")
	}()
	go func() {
		defer wg.Done()
		errs[3] = s.userService.Get().UploadCreatorWithUserId(ctx, params.reqID, params.userID, params.username)
		logger.Debug("upload creator with user id done!")
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			http.Error(w, "error: " + err.Error(), http.StatusInternalServerError)
			return
		}
	}
	response := fmt.Sprintf("success! user %s (id=%d) composed post: %s\n", params.username, params.userID, params.text)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response))
}

type readTimelineParams struct {
	reqID 		int64
	userID 	int64
	start 		int64
	stop 		int64
}

func genReadTimeline() readTimelineParams {
	params := readTimelineParams {
		reqID: 	rand.New(rand.NewSource(time.Now().UnixNano())).Int63(),
		userID: 0,
		start:  0,
		stop:   time.Now().UnixMilli(),
	}
	return params
}

func (s *server) readHomeTimelineHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering wkr2-api/home-timeline/read")

	params := genReadTimeline()
	posts, err := s.userTimelineService.Get().ReadUserTimeline(ctx, params.reqID, params.userID, params.start, params.stop)
	if err != nil {
		http.Error(w, "error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	response, err := json.Marshal(posts)
	if err != nil {
		http.Error(w, "error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(response)
}

func (s *server) readUserTimelineHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.Logger(ctx)
	logger.Info("entering wkr2-api/user-timeline/read")
	
	params := genReadTimeline()
	posts, err := s.homeTimelineService.Get().ReadHomeTimeline(ctx, params.reqID, params.userID, params.start, params.stop)
	if err != nil {
		http.Error(w, "error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	response, err := json.Marshal(posts)
	if err != nil {
		http.Error(w, "error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(response)
}
