package system

import (
	"github.com/NikaSir/vless-gateway/internal/model"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type Collector struct {
	ReadFile  func(string) ([]byte, error)
	FreeBytes func() (uint64, error)
}

func New(root string) Collector {
	return Collector{ReadFile: os.ReadFile, FreeBytes: func() (uint64, error) {
		var v syscall.Statfs_t
		e := syscall.Statfs(root, &v)
		return v.Bavail * uint64(v.Bsize), e
	}}
}
func (c Collector) metric(path string, scale float64) model.Metric[float64] {
	if c.ReadFile == nil {
		return model.Unavailable[float64]("not_measured")
	}
	b, e := c.ReadFile(path)
	if e != nil {
		return model.Unavailable[float64]("not_available")
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return model.Unavailable[float64]("invalid_metric")
	}
	v, e := strconv.ParseFloat(fields[0], 64)
	if e != nil || v < 0 {
		return model.Unavailable[float64]("invalid_metric")
	}
	return model.Available(v / scale)
}
func (c Collector) Collect() model.SystemSnapshot {
	s := model.SystemSnapshot{Uptime: c.metric("/proc/uptime", 1), Load: c.metric("/proc/loadavg", 1), Temperature: c.metric("/sys/class/thermal/thermal_zone0/temp", 1000), StorageFree: model.Unavailable[uint64]("not_measured"), SingBoxVersion: model.Unavailable[string]("not_observed")}
	if c.FreeBytes != nil {
		v, e := c.FreeBytes()
		if e == nil {
			s.StorageFree = model.Available(v)
		}
	}
	return s
}
