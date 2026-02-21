package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveWSConnections tracks currently connected WebSocket clients.
	ActiveWSConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "woongkie_active_ws_connections",
		Help: "Number of currently active WebSocket connections.",
	})

	// MessagesTotal counts total chat messages processed.
	MessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "woongkie_messages_total",
		Help: "Total number of chat messages processed.",
	})

	// RoomsActive tracks how many rooms currently have an active hub.
	RoomsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "woongkie_rooms_active",
		Help: "Number of rooms with an active WebSocket hub.",
	})

	// MessagesDropped counts messages dropped due to a full insertQueue.
	MessagesDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "woongkie_messages_dropped_total",
		Help: "Total number of chat messages dropped because insertQueue was full.",
	})

	// RedisMessagesDropped counts Redis Pub/Sub messages dropped due to a slow consumer.
	RedisMessagesDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "woongkie_redis_messages_dropped_total",
		Help: "Total number of Redis Pub/Sub messages dropped because the consumer was too slow.",
	})
)
