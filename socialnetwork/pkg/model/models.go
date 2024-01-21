package model

import "github.com/ServiceWeaver/weaver"

type Message struct {
	ReqID  			int64 	`json:"reqid"`
	PostID 			int64 	`json:"postid"`
	Timestamp 		int64 	`json:"timestamp"`
	UserMentionIDs 	[]int64 `json:"usermentionids"`
}

type Creator struct {
	weaver.AutoMarshal
	UserID 		int64 	`bson:"user_id"`
	Username 	string  `bson:"username"`
}

type Media struct {
	weaver.AutoMarshal
	MediaID 	int64  	 	`bson:"media_id"`
	MediaType 	string `bson:"media_type"`
}

type URL struct {
	weaver.AutoMarshal
	ExpandedUrl 	string 	`bson:"expanded_url"`
	ShortenedUrl 	string 	`bson:"shortened_url"`
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
