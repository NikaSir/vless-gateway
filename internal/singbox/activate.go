package singbox

import (
	"context"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/state"
	"os"
	"path/filepath"
	"time"
)

type Checker interface {
	Check(context.Context, string) error
}
type Reloader interface {
	Reload(context.Context) error
	Healthy(context.Context) error
}

func Activate(ctx context.Context, target string, data []byte, c Checker, r Reloader) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	dir := filepath.Dir(target)
	if os.MkdirAll(dir, 0700) != nil {
		return errors.New("config directory unavailable")
	}
	stage, e := os.CreateTemp(dir, ".config-stage-")
	if e != nil {
		return errors.New("config staging failed")
	}
	name := stage.Name()
	_ = stage.Close()
	defer os.Remove(name)
	if state.AtomicWrite(name, data, 0600) != nil {
		return errors.New("config staging failed")
	}
	if c.Check(ctx, name) != nil {
		return errors.New("config validation failed")
	}
	previous, e := os.ReadFile(target)
	hadOld := e == nil
	if e != nil && !os.IsNotExist(e) {
		return errors.New("prior config unreadable")
	}
	if hadOld {
		if state.AtomicWrite(target+".previous", previous, 0600) != nil {
			return errors.New("config backup failed")
		}
	}
	if e = state.AtomicWrite(target, data, 0600); e != nil {
		return errors.New("config publication failed")
	}
	if r.Reload(ctx) == nil && r.Healthy(ctx) == nil {
		return nil
	}
	rollbackCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if hadOld {
		if state.AtomicWrite(target, previous, 0600) != nil {
			return errors.New("config rollback write failed")
		}
	} else {
		_ = os.Remove(target)
	}
	_ = r.Reload(rollbackCtx)
	return errors.New("config activation failed; previous generation restored")
}
