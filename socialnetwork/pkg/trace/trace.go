package trace

import (
	"github.com/ServiceWeaver/weaver"

	"go.opentelemetry.io/otel/trace"
)

type SpanContext struct {
	weaver.AutoMarshal
	TraceID 	[16]byte `json:"trace_id"`
	SpanID 		[8]byte `json:"span_id"`
	TraceFlags 	byte 	`json:"trace_flags"`
	TraceState 	string 	`json:"trace_state"`
	Remote 		bool 	`json:"remote"`
}

func ParseSpanContext(sc SpanContext) (trace.SpanContext, error) {
	traceState, err := trace.ParseTraceState(sc.TraceState)
	if err != nil {
		return trace.SpanContext{}, err
	}
	config := trace.SpanContextConfig{
		TraceID:    sc.TraceID,
		SpanID:     sc.SpanID,
		TraceFlags: trace.TraceFlags(sc.TraceFlags),
		TraceState: traceState,
		Remote: 	sc.Remote,
	}
	return trace.NewSpanContext(config), nil
}

func BuildSpanContext(sc trace.SpanContext) SpanContext {
	return SpanContext{
		TraceID:    sc.TraceID(),
		SpanID:     sc.SpanID(),
		TraceFlags: byte(sc.TraceFlags()),
		TraceState: sc.TraceState().String(),
		Remote: 	sc.IsRemote(),
	}
}
