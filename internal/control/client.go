package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"
)

func Call(ctx context.Context, path string, req Request) (Response, error) {
	var r Response
	if !valid(req) {
		return r, errors.New("invalid local command")
	}
	d := net.Dialer{Timeout: time.Second}
	c, e := d.DialContext(ctx, "unix", path)
	if e != nil {
		return r, errors.New("gatewayd control unavailable")
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(30 * time.Second))
	if json.NewEncoder(c).Encode(req) != nil {
		return r, errors.New("control send failed")
	}
	if json.NewDecoder(io.LimitReader(c, 1<<20)).Decode(&r) != nil {
		return r, errors.New("invalid control response")
	}
	if !r.OK {
		return r, errors.New("local command failed")
	}
	return r, nil
}
