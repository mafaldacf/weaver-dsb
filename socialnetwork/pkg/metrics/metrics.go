package metrics

import "github.com/ServiceWeaver/weaver/metrics"

type ClientRequestLabels struct {
    Region string
}

var (
	ClientRequest = metrics.NewCounterMap[ClientRequestLabels](
		"sn_client_request",
		"Number of client requests received at EU or US",
	)

	// wrk2 api
	ComposePostDuration = metrics.NewHistogram(
		"sn_compose_post_duration_ms",
		"Duration of compose post endpoint in milliseconds",
		metrics.NonNegativeBuckets,
	)
	// composed post service
	ComposedPosts = metrics.NewCounter(
		"sn_composed_posts",
		"The number of composed posts",
	)
	// post storage service
	WritePostDurationMs = metrics.NewHistogram(
		"sn_write_post_duration_ms",
		"Duration of queue in milliseconds",
		metrics.NonNegativeBuckets,
	)
	// write home timeline service
	QueueDurationMs = metrics.NewHistogram(
		"sn_queue_duration_ms",
		"Duration of queue in milliseconds",
		metrics.NonNegativeBuckets,
	)
	ReceivedNotifications = metrics.NewCounter(
		"sn_received_notifications",
		"The number of received notifications",
	)
	Inconsistencies = metrics.NewCounter(
		"sn_inconsistencies",
		"The number of times an cross-service inconsistency has occured",
	)
)
