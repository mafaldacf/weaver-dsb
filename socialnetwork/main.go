//go:generate weaver generate ./pkg/wrk2 . ./pkg/services ./pkg/model ./pkg/trace ./pkg/metrics

package main

import (
	"context"
	"log"

	"socialnetwork/pkg/wrk2"

	"github.com/ServiceWeaver/weaver"
)

func main() {
	if err := weaver.Run(context.Background(), wrk2.Serve); err != nil {
		log.Fatal(err)
	}
}
