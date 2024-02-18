package services

import (
	"context"
	"math/rand"

	"socialnetwork/pkg/model"
	"socialnetwork/pkg/storage"
	"socialnetwork/pkg/utils"

	"github.com/ServiceWeaver/weaver"
	"github.com/bradfitz/gomemcache/memcache"
	"go.mongodb.org/mongo-driver/mongo"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

type UrlShortenService interface {
	UploadUrls(ctx context.Context, reqID int64, urls []string) error
	GetExtendedUrls(ctx context.Context, reqID int64, shortenedUrls []string) ([]string, error)
}

type urlShortenService struct {
	weaver.Implements[UrlShortenService]
	weaver.WithConfig[urlShortenServiceOptions]
	composePostService weaver.Ref[ComposePostService]
	mongoClient        *mongo.Client
	memCachedClient    *memcache.Client
	hostname           string
}

type urlShortenServiceOptions struct {
	MongoDBAddr 	map[string]string 	`toml:"mongodb_address"`
	MemCachedAddr 	map[string]string 	`toml:"memcached_address"`
	MongoDBPort 	map[string]int    	`toml:"mongodb_port"`
	MemCachedPort 	map[string]int    	`toml:"memcached_port"`
	Region 			string
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

	region, err := utils.Region()
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	u.Config().Region = region

	u.mongoClient, err = storage.MongoDBClient(ctx, u.Config().MongoDBAddr[region], u.Config().MongoDBPort[region])
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	u.memCachedClient = storage.MemCachedClient(u.Config().MemCachedAddr[region], u.Config().MemCachedPort[region])
	logger.Info("url shorten service running!", "region", u.Config().Region,
		"mongodb_addr", u.Config().MongoDBAddr[region], "mongodb_port", u.Config().MongoDBPort[region],
		"memcached_addr", u.Config().MemCachedAddr[region], "memcached_port", u.Config().MemCachedPort[region],
	)
	return nil
}

func (u *urlShortenService) UploadUrls(ctx context.Context, reqID int64, urls []string) error {
	logger := u.Logger(ctx)
	logger.Debug("entering upload urls", "req_id", reqID, "urls", urls)

	var targetUrls []model.URL
	var targetUrl_docs []interface{}
	for _, url := range urls {
		targetUrl := model.URL{
			ExpandedUrl:  url,
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

func (u *urlShortenService) GetExtendedUrls(ctx context.Context, reqID int64, shortenedUrls []string) ([]string, error) {
	// not implemented in original dsb
	return nil, nil
}
