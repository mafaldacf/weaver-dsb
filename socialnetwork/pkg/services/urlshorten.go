package services

import (
	"context"
	"math/rand"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"

	"github.com/ServiceWeaver/weaver"
	"go.mongodb.org/mongo-driver/mongo"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

type UrlShortenService interface {
	UploadUrls(ctx context.Context, reqID int64, urls []string) error
}

type urlShortenService struct {
	weaver.Implements[UrlShortenService]
	weaver.WithConfig[urlShortenServiceOptions]
	composePostService  weaver.Ref[ComposePostService]
	mongoClient 		*mongo.Client
	hostname 			string
}

type urlShortenServiceOptions struct {
	MongoDBAddr string `toml:"mongodb_address"`
	MongoDBPort int    `toml:"mongodb_port"`
}

func (u *urlShortenService) genRandomStr(length int) string {
	b := make([]rune, length)
    for i := range b {
        b[i] = letterRunes[rand.Intn(len(letterRunes))]
    }
    return string(b)
}


func (u *urlShortenService) Init(ctx context.Context) error {
	logger := u.Logger(ctx)
	var err error
	u.mongoClient, err = storage.MongoDBClient(ctx, u.Config().MongoDBAddr, u.Config().MongoDBPort)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	logger.Info("url shorten service running!", "mongodb_addr", u.Config().MongoDBAddr, "mongodb_port", u.Config().MongoDBPort)
	return nil
}

func (u *urlShortenService) UploadUrls(ctx context.Context, reqID int64, urls []string) error {
	logger := u.Logger(ctx)
	logger.Debug("entering upload urls", "req_id", reqID, "urls", urls)
	
	var targetUrls []model.URL
	var targetUrl_docs []interface{}
	for _, url := range urls {
		targetUrl := model.URL {
			ExpandedUrl: url,
			ShortenedUrl: u.hostname + u.genRandomStr(10), 
		}
		targetUrls = append(targetUrls, targetUrl)
		targetUrl_docs = append(targetUrl_docs, targetUrl)
	}

	if len(targetUrls) > 0 {
		collection := u.mongoClient.Database("url-shorten").Collection("url-shorten")
		_, err := collection.InsertMany(ctx, targetUrl_docs)
		if err != nil {
			logger.Error("error inserting target urls in mongodb", "msg", err.Error())
			return err
		}
	}

	return u.composePostService.Get().UploadUrls(ctx, reqID, targetUrls)
}
