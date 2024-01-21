package services

import "context"

type SocialGraph interface {
	GetFollowers(ctx context.Context, reqID int64, userID int64) ([]int64, error)
	GetFollowees(ctx context.Context, reqID int64, userID int64) ([]int64, error)
	Follow(ctx context.Context, reqID int64, userID int64, followeeID int64) error
	Unfollow(ctx context.Context, reqID int64, userID int64, followeeID int64) error
	FollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error
	UnfollowWithUsername(ctx context.Context, reqID int64, userUsername string, followeeUsername string) error
	InsertUser(ctx context.Context, reqID int64, userID int64) error

}
