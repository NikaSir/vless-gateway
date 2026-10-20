package clash

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type client struct {
	base, secret string
	http         *http.Client
}

func New(base, secret string, h *http.Client) (Client, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid Clash API address")
	}
	if strings.ContainsAny(secret, "\r\n") || secret == "" {
		return nil, errors.New("invalid Clash API token")
	}
	if h == nil {
		h = &http.Client{Timeout: 6 * time.Second}
	}
	hc := *h
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client{strings.TrimRight(base, "/"), secret, &hc}, nil
}
func (c *client) request(ctx context.Context, method, path string, body any, out any, stream bool) error {
	var b io.Reader
	if body != nil {
		data, e := json.Marshal(body)
		if e != nil {
			return errors.New("Clash request encoding failed")
		}
		b = bytes.NewReader(data)
	}
	req, e := http.NewRequestWithContext(ctx, method, c.base+path, b)
	if e != nil {
		return errors.New("Clash request creation failed")
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Content-Type", "application/json")
	resp, e := c.http.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("Clash request unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("Clash request rejected")
	}
	if out == nil {
		return nil
	}
	limited := io.LimitReader(resp.Body, (1<<20)+1)
	if stream {
		if json.NewDecoder(limited).Decode(out) != nil {
			return errors.New("invalid Clash stream")
		}
		return nil
	}
	data, e := io.ReadAll(limited)
	if e != nil || len(data) > 1<<20 || json.Unmarshal(data, out) != nil {
		return errors.New("invalid Clash response")
	}
	return nil
}
func (c *client) Delay(ctx context.Context, outbound, target string, timeout time.Duration) (DelayResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout+time.Second)
	defer cancel()
	q := url.Values{"url": {target}, "timeout": {strconv.FormatInt(timeout.Milliseconds(), 10)}}
	var d struct {
		Delay *int64 `json:"delay"`
	}
	e := c.request(ctx, "GET", "/proxies/"+url.PathEscape(outbound)+"/delay?"+q.Encode(), nil, &d, false)
	if e != nil {
		return DelayResult{}, e
	}
	if d.Delay == nil || *d.Delay < 0 || *d.Delay > timeout.Milliseconds() {
		return DelayResult{}, errors.New("invalid probe delay")
	}
	return DelayResult{time.Duration(*d.Delay) * time.Millisecond}, nil
}
func (c *client) Selected(ctx context.Context, selector string) (string, error) {
	var d struct {
		Now string `json:"now"`
	}
	e := c.request(ctx, "GET", "/proxies/"+url.PathEscape(selector), nil, &d, false)
	if e == nil && d.Now == "" {
		e = errors.New("missing selector state")
	}
	return d.Now, e
}
func (c *client) Select(ctx context.Context, selector, outbound string) error {
	return c.request(ctx, "PUT", "/proxies/"+url.PathEscape(selector), map[string]string{"name": outbound}, nil, false)
}
func (c *client) Traffic(ctx context.Context) (TrafficSnapshot, error) {
	var v TrafficSnapshot
	e := c.request(ctx, "GET", "/traffic", nil, &v, true)
	return v, e
}
func (c *client) Connections(ctx context.Context) (ConnectionSnapshot, error) {
	var v struct {
		DownloadTotal uint64            `json:"downloadTotal"`
		UploadTotal   uint64            `json:"uploadTotal"`
		Connections   []json.RawMessage `json:"connections"`
	}
	e := c.request(ctx, "GET", "/connections", nil, &v, false)
	return ConnectionSnapshot{v.DownloadTotal, v.UploadTotal, len(v.Connections)}, e
}
func (c *client) Version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"version"`
	}
	e := c.request(ctx, "GET", "/version", nil, &v, false)
	return v.Version, e
}
