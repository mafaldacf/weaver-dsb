package model

import "github.com/ServiceWeaver/weaver"

type Message struct {
	ReqID  			int64 	`json:"req_id"`
	UserID 			int64 	`json:"user_id"`
	PostID 			int64 	`json:"post_id"`
	Timestamp 		int64 	`json:"timestamp"`
	UserMentionIDs 	[]int64 `json:"user_mention_ids"`
}

type Creator struct {
	weaver.AutoMarshal
	UserID 		int64 	`bson:"user_id"`
	Username 	string  `bson:"username"`
}

type Media struct {
	weaver.AutoMarshal
	MediaID 	int64  `bson:"media_id"`
	MediaType 	string `bson:"media_type"`
}

type URL struct {
	weaver.AutoMarshal
	ExpandedUrl 	string 	`bson:"expanded_url"`
	ShortenedUrl 	string 	`bson:"shortened_url"`
}

type User struct {
	weaver.AutoMarshal
	UserID 		int64 	`bson:"user_id"`
	FirstName 	string 	`bson:"first_name"`
	LastName 	string 	`bson:"last_name"`
	Username 	string  `bson:"username"`
	PwdHashed 	string  `bson:"pwd_hashed"`
	Salt 		string  `bson:"salt"`
}

type UserMention struct {
	weaver.AutoMarshal
	UserID 		int64 	`bson:"user_id"`
	Username 	string 	`bson:"username"`
}

type PostType int

const (
    POST_TYPE_POST PostType = iota 		// 0
    POST_TYPE_REPOST 					// 1
    POST_TYPE_REPLY 					// 2
    POST_TYPE_DM 						// 3
)

type Post struct {
	// make post serializable
	// by default, struct literal types are not serializable
	weaver.AutoMarshal
	PostID 			int64  			`bson:"post_id"`
	ReqID  			int64  			`bson:"req_id"`
	Creator 		Creator 		`bson:"creator"`
	Text   			string 			`bson:"text"`
	UserMentions 	[]UserMention 	`bson:"user_mentions"`
	Media 			[]Media 		`bson:"media"`
	URLs 			[]URL 			`bson:"urls"`
	Timestamp 		int64 			`bson:"timestamp"`
	PostType 		PostType 		`bson:"posttype"`
}

type TimelinePostInfo struct {
	PostID    int64 `bson:"post_id"`
	Timestamp int64 `bson:"timestamp"`
}

type Timeline struct {
	UserID int64
	Posts  []TimelinePostInfo
}
