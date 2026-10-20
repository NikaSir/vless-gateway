package state

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/model"
	"os"
	"path/filepath"
	"time"
)

type Store struct{ root string }

func New(root string) *Store { return &Store{root} }

// AtomicWrite persists an entire generation on one filesystem before publication.
func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".pending-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func (s *Store) Save(ctx context.Context, v model.StatusSnapshot) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(v.History) > 128 {
		v.History = v.History[len(v.History)-128:]
	}
	b, err := json.Marshal(v)
	if err != nil {
		return errors.New("state encode failed")
	}
	return AtomicWrite(filepath.Join(s.root, "status.json"), b, 0600)
}
func (s *Store) Load(ctx context.Context) (model.StatusSnapshot, error) {
	var v model.StatusSnapshot
	if ctx.Err() != nil {
		return v, ctx.Err()
	}
	path := filepath.Join(s.root, "status.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	if len(b) > 1<<20 || json.Unmarshal(b, &v) != nil {
		_ = os.Rename(path, path+".corrupt-"+time.Now().UTC().Format("20060102T150405.000000000"))
		return v, errors.New("invalid saved state")
	}
	return v, nil
}
