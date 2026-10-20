package singbox

import (
	"context"
	"encoding/json"
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
	if _, e := recoverRuntime(ctx, target, r); e != nil {
		return e
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
	marker, _ := json.Marshal(configTransaction{Format: 1, HadOld: hadOld})
	if state.AtomicWrite(target+".transaction", marker, 0600) != nil {
		return errors.New("config journal write failed")
	}
	if e = state.AtomicWrite(target, data, 0600); e != nil {
		return errors.New("config publication failed")
	}
	if r.Reload(ctx) == nil && r.Healthy(ctx) == nil {
		return clearTransaction(target)
	}
	rollbackCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, e := recoverRuntime(rollbackCtx, target, r); e != nil {
		return errors.New("config activation failed; " + e.Error())
	}
	return errors.New("config activation failed; previous generation restored")
}

type configTransaction struct {
	Format int  `json:"format"`
	HadOld bool `json:"had_old"`
}

// Recover performs disk-only recovery before ordinary process startup.
// Live recovery uses recoverRuntime, retaining its journal until acknowledgement.
func Recover(target string) (bool, error) {
	recovered, e := restorePendingConfig(target)
	if e != nil || !recovered {
		return recovered, e
	}
	if e := clearTransaction(target); e != nil {
		return false, e
	}
	return true, nil
}

func recoverRuntime(ctx context.Context, target string, r Reloader) (bool, error) {
	recovered, e := restorePendingConfig(target)
	if e != nil || !recovered {
		return recovered, e
	}
	if r.Reload(ctx) != nil {
		return false, errors.New("config rollback reload failed")
	}
	if r.Healthy(ctx) != nil {
		return false, errors.New("config rollback health not confirmed")
	}
	if e := clearTransaction(target); e != nil {
		return false, e
	}
	return true, nil
}

// Restore durable bytes without committing recovery. This protects .previous
// across failed live rollback attempts and recovery retries.
func restorePendingConfig(target string) (bool, error) {
	b, e := os.ReadFile(target + ".transaction")
	if os.IsNotExist(e) {
		return false, nil
	}
	if e != nil {
		return false, errors.New("config journal unreadable")
	}
	var transaction configTransaction
	if len(b) > 4096 || json.Unmarshal(b, &transaction) != nil || transaction.Format != 1 {
		return false, errors.New("config journal invalid")
	}
	if transaction.HadOld {
		old, e := os.ReadFile(target + ".previous")
		if e != nil {
			return false, errors.New("known-good config unavailable")
		}
		if state.AtomicWrite(target, old, 0600) != nil {
			return false, errors.New("config recovery write failed")
		}
	} else {
		if e := os.Remove(target); e != nil && !os.IsNotExist(e) {
			return false, errors.New("config recovery remove failed")
		}
		if e := syncConfigDirectory(target); e != nil {
			return false, e
		}
	}
	return true, nil
}

func clearTransaction(target string) error {
	if e := os.Remove(target + ".transaction"); e != nil && !os.IsNotExist(e) {
		return errors.New("config journal removal failed")
	}
	return syncConfigDirectory(target)
}

func syncConfigDirectory(target string) error {
	dir, e := os.Open(filepath.Dir(target))
	if e != nil {
		return errors.New("config directory unavailable")
	}
	defer dir.Close()
	if dir.Sync() != nil {
		return errors.New("config journal flush failed")
	}
	return nil
}
