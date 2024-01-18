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
	UserID 		int64
	Username 	string 
}

type Media struct {
	weaver.AutoMarshal
}

type URL struct {
	weaver.AutoMarshal
}

type UserMention struct {
	weaver.AutoMarshal
	UserID 		int64
	Username 	string
}

type PostType int

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
