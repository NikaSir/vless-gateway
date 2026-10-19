package control

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestControlSocketPermissionProtocolAndStaleGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, e := Listen(path)
	if e != nil {
		if os.Getenv("NIKAS_ALLOW_SOCKET_SKIP") == "1" && errors.Is(e, syscall.EPERM) {
			t.Skip("runtime forbids AF_UNIX; required CI test remains enabled")
		}
		t.Fatal(e)
	}
	defer l.Close()
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("unsafe socket mode")
	}
	go Serve(ctx, l, func(ctx context.Context, r Request) Response {
		return Response{OK: true, Data: map[string]string{"mode": r.Mode}}
	})
	r, e := Call(ctx, path, Request{Command: "set-mode", Mode: "direct"})
	if os.Geteuid() == 0 && (e != nil || !r.OK) {
		t.Fatal(r, e)
	}
	if os.Geteuid() != 0 && e == nil {
		t.Fatal("non-root peer accepted")
	}
	if _, e = Listen(path); e == nil {
		t.Fatal("active socket replaced")
	}
	conn, e := net.Dial("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_, _ = conn.Write([]byte("{\"command\":\"status\",\"extra\":true}\n"))
	var b [256]byte
	n, _ := conn.Read(b[:])
	_ = conn.Close()
	if n == 0 {
		t.Fatal("missing error response")
	}
	file := filepath.Join(t.TempDir(), "regular")
	_ = os.WriteFile(file, []byte("keep"), 0600)
	if _, e = Listen(file); e == nil {
		t.Fatal("regular file replaced")
	}
}
