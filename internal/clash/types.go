package clash

import (
	"context"
	"time"
)

type DelayResult struct{ Latency time.Duration }
type TrafficSnapshot struct {
	Up   uint64 `json:"up"`
	Down uint64 `json:"down"`
}
type ConnectionSnapshot struct {
	DownloadTotal uint64
	UploadTotal   uint64
	Active        int
}
type Client interface {
	Delay(context.Context, string, string, time.Duration) (DelayResult, error)
	Selected(context.Context, string) (string, error)
	Select(context.Context, string, string) error
	Traffic(context.Context) (TrafficSnapshot, error)
	Connections(context.Context) (ConnectionSnapshot, error)
	Version(context.Context) (string, error)
}
