//go:generate weaver generate ./pkg/wrk2 . ./pkg/services ./pkg/model ./pkg/trace ./pkg/metrics

package main

import (
	"context"
	"log"

	"socialnetwork/pkg/wrk2"

	"github.com/ServiceWeaver/weaver"
)

// this is an entry file for socialnetwork application
// the source code of services is in the "pkg" folder
func main() {
	if err := weaver.Run(context.Background(), wrk2.Serve); err != nil {
		log.Fatal(err)
	}
}
