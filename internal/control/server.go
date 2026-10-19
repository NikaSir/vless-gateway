package control

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func Listen(path string) (net.Listener, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	if f, e := os.Lstat(path); e == nil {
		if f.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("control path is not a socket")
		}
		c, e := net.DialTimeout("unix", path, 200*time.Millisecond)
		if e == nil {
			_ = c.Close()
			return nil, errors.New("control socket already active")
		}
		if os.Remove(path) != nil {
			return nil, errors.New("stale socket cleanup failed")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	l, e := net.Listen("unix", path)
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		_ = l.Close()
		return nil, e
	}
	return l, nil
}
func rootPeer(conn net.Conn) bool {
	u, ok := conn.(*net.UnixConn)
	if !ok {
		return false
	}
	raw, e := u.SyscallConn()
	if e != nil {
		return false
	}
	uid := uint32(1)
	e = raw.Control(func(fd uintptr) {
		cred, e := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if e == nil {
			uid = cred.Uid
		}
	})
	return e == nil && uid == 0
}
func Serve(ctx context.Context, l net.Listener, handler func(context.Context, Request) Response) error {
	go func() { <-ctx.Done(); _ = l.Close() }()
	for {
		c, e := l.Accept()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		go func() {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(30 * time.Second))
			if !rootPeer(c) {
				_ = json.NewEncoder(c).Encode(Response{Error: "root_required"})
				return
			}
			scanner := bufio.NewScanner(io.LimitReader(c, 65537))
			scanner.Buffer(make([]byte, 1024), 65536)
			if !scanner.Scan() {
				return
			}
			var r Request
			d := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
			d.DisallowUnknownFields()
			if d.Decode(&r) != nil || d.Decode(new(any)) != io.EOF || !valid(r) {
				_ = json.NewEncoder(c).Encode(Response{Error: "invalid_command"})
				return
			}
			rc, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()
			resp := handler(rc, r)
			_ = json.NewEncoder(c).Encode(resp)
		}()
	}
}
