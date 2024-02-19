package metrics

import "github.com/ServiceWeaver/weaver/metrics"

type RegionLabel struct {
    Region string
}

var (
	// wrk2 api
	ComposePostDuration = metrics.NewHistogramMap[RegionLabel](
		"sn_compose_post_duration_ms",
		"Duration of compose post endpoint in milliseconds in the current region",
		metrics.NonNegativeBuckets,
	)
	// composed post service
	ComposedPosts = metrics.NewCounterMap[RegionLabel](
		"sn_composed_posts",
		"The number of composed posts in the current region",
	)
	// post storage service
	WritePostDurationMs = metrics.NewHistogramMap[RegionLabel](
		"sn_write_post_duration_ms",
		"Duration of queue in milliseconds in the current region",
		metrics.NonNegativeBuckets,
	)
	// write home timeline service
	QueueDurationMs = metrics.NewHistogramMap[RegionLabel](
		"sn_queue_duration_ms",
		"Duration of queue in milliseconds in the current region",
		metrics.NonNegativeBuckets,
	)
	ReceivedNotifications = metrics.NewCounterMap[RegionLabel](
		"sn_received_notifications",
		"The number of received notifications in the current region",
	)
	Inconsistencies = metrics.NewCounterMap[RegionLabel](
		"sn_inconsistencies",
		"The number of times an cross-service inconsistency has occured in the current region",
	)
)
