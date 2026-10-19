package provision

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func Inspect(ctx context.Context) (Inventory, error) {
	var i Inventory
	i.Arch = runtime.GOARCH
	get := func(name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		b, e := cmd.Output()
		if e != nil {
			return nil, errors.New("preflight dependency unavailable")
		}
		if len(b) > 8<<20 {
			return nil, errors.New("preflight response too large")
		}
		return b, nil
	}
	b, e := get("ubus", "call", "system", "board")
	if e != nil {
		return i, e
	}
	var board struct {
		Board   string `json:"board_name"`
		Release struct {
			Distribution string `json:"distribution"`
			Version      string `json:"version"`
		} `json:"release"`
	}
	if json.Unmarshal(b, &board) != nil {
		return i, errors.New("invalid board inventory")
	}
	i.Board = board.Board
	i.Distribution = board.Release.Distribution
	i.Version = board.Release.Version
	b, e = get("sing-box", "version")
	if e != nil {
		return i, e
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "sing-box version ") {
			i.SingBoxVersion = strings.TrimPrefix(line, "sing-box version ")
		}
		if strings.Contains(line, "with_clash_api") {
			i.ClashSupport = true
		}
	}
	b, e = get("ip", "-j", "link", "show")
	if e != nil {
		return i, e
	}
	if json.Unmarshal(b, &i.Links) != nil {
		return i, errors.New("invalid link inventory")
	}
	b, e = get("ip", "-j", "-4", "route", "show", "table", "all")
	if e != nil {
		return i, e
	}
	if json.Unmarshal(b, &i.Routes) != nil {
		return i, errors.New("invalid route inventory")
	}
	b, e = get("ip", "-j", "-4", "rule", "show")
	if e != nil {
		return i, e
	}
	var rules []struct {
		Priority int `json:"priority"`
	}
	if json.Unmarshal(b, &rules) != nil {
		return i, errors.New("invalid rule inventory")
	}
	for _, r := range rules {
		i.RulePriorities = append(i.RulePriorities, r.Priority)
	}
	b, e = get("nft", "-j", "list", "tables")
	if e != nil {
		return i, e
	}
	var nft struct {
		NFTables []struct {
			Table *struct {
				Family string `json:"family"`
				Name   string `json:"name"`
			} `json:"table"`
		} `json:"nftables"`
	}
	if json.Unmarshal(b, &nft) != nil {
		return i, errors.New("invalid firewall inventory")
	}
	for _, v := range nft.NFTables {
		if v.Table != nil {
			i.NFTTables = append(i.NFTTables, v.Table.Family+" "+v.Table.Name)
		}
	}
	return i, nil
}
