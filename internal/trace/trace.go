package trace

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/xid"
)

const (
	HTTPHeader = "X-Trace-Id"
	NATSHeader = "Nats-Trace-Id"
	LogKey     = "trace_id"
)

func NewID() string {
	return xid.New().String()
}

// FromNATSHeaders extracts a trace ID from NATS message headers,
// generating a new one if absent.
func FromNATSHeaders(headers nats.Header) string {
	if headers != nil {
		if id := headers.Get(NATSHeader); id != "" {
			return id
		}
	}
	return NewID()
}
