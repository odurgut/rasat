// Package stream fans ingest trace summaries to WebSocket subscribers.
//
// Publish never waits on a client. A full per-subscriber buffer drops that
// client so OTLP ingest is not delayed. AfterWrite may also drop events that
// exceed RASAT_STREAM_MAX_PER_SEC so a live UI is not a 10k-msg/s firehose.
package stream
