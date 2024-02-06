package storage

import (
	"fmt"

	"github.com/bradfitz/gomemcache/memcache"
)

func MemCachedClient(address string, port int) *memcache.Client {
	uri := fmt.Sprintf("%s:%d", address, port)
	client := memcache.New(uri)
	client.MaxIdleConns = 1000
	return client
}
