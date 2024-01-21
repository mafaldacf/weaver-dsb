package services

import (
	"context"

	"github.com/ServiceWeaver/weaver"
)

type UserService interface {
	Login(ctx context.Context, reqID int64, username string, password string) (string, error)
	RegisterUserWithId(ctx context.Context, reqID int64, firstName string, lastName string, username string, password string, userID int64) error
	RegisterUser(ctx context.Context, reqID int64, firstName string, lastName string, username string, password string) error
	UploadCreatorWithUserId(ctx context.Context, reqID int64, userID int64, username string) error
	UploadCreatorWithUsername(ctx context.Context, reqID int64, username string) error
	GetUserId(ctx context.Context, reqID int64, username string) (int64, error)
}

type userService struct {
	weaver.Implements[UserService]
	weaver.WithConfig[userServiceOptions]
	socialGraphService   weaver.Ref[SocialGraphService]
	composePostService   weaver.Ref[ComposePostService]
}

type userServiceOptions struct {
}
