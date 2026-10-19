package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func ReadJSON(path string, out any) error {
	f, e := os.Open(path)
	if e != nil {
		return errors.New("configuration file unavailable")
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if e != nil || len(b) > 1<<20 {
		return errors.New("configuration file too large")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return errors.New("invalid configuration structure")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("invalid trailing configuration data")
	}
	return nil
}
func LoadControl(root string) (Runtime, error) {
	r := Runtime{Root: root}
	r.Settings = Settings{TransitSubnet: "192.168.2.0/24", IntervalSeconds: 15, TimeoutSeconds: 5, BootstrapSuccesses: 2, FailureCycles: 3, RecoveryCycles: 8}
	for _, entry := range []struct {
		name string
		dst  any
	}{{"settings.json", &r.Settings}, {"hardware.json", &r.Hardware}, {"auth.json", &r.Auth}} {
		if e := ReadJSON(filepath.Join(root, entry.name), entry.dst); e != nil {
			return Runtime{}, errors.New(entry.name + ": " + e.Error())
		}
	}
	return r, nil
}
func Load(root string) (Runtime, error) {
	r, e := LoadControl(root)
	if e != nil {
		return r, e
	}
	if e = ReadJSON(filepath.Join(root, "secrets.json"), &r.Secrets); e != nil {
		return Runtime{}, errors.New("secrets.json: invalid or unavailable")
	}
	return r, nil
}
