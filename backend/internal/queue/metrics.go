package queue

import "context"

type JetStreamStatsProvider interface {
	JetStreamSnapshot(context.Context) (JetStreamSnapshot, error)
}

type JetStreamSnapshot struct {
	Enabled   bool                     `json:"enabled"`
	LastError string                   `json:"last_error,omitempty"`
	Streams   []JetStreamStreamStats   `json:"streams,omitempty"`
	Consumers []JetStreamConsumerStats `json:"consumers,omitempty"`
}

type JetStreamStreamStats struct {
	Name          string   `json:"name"`
	Subjects      []string `json:"subjects,omitempty"`
	Messages      uint64   `json:"messages"`
	Bytes         uint64   `json:"bytes"`
	FirstSequence uint64   `json:"first_sequence"`
	LastSequence  uint64   `json:"last_sequence"`
	ConsumerCount int      `json:"consumer_count"`
	LastError     string   `json:"last_error,omitempty"`
}

type JetStreamConsumerStats struct {
	Stream                    string `json:"stream"`
	Name                      string `json:"name"`
	FilterSubject             string `json:"filter_subject,omitempty"`
	Pending                   uint64 `json:"pending"`
	AckPending                int    `json:"ack_pending"`
	Redelivered               int    `json:"redelivered"`
	NumWaiting                int    `json:"num_waiting"`
	MaxAckPending             int    `json:"max_ack_pending"`
	AckWaitSeconds            int64  `json:"ack_wait_seconds"`
	DeliveredConsumerSequence uint64 `json:"delivered_consumer_sequence"`
	AckFloorConsumerSequence  uint64 `json:"ack_floor_consumer_sequence"`
	LastError                 string `json:"last_error,omitempty"`
}
