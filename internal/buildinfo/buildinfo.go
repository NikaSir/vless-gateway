package buildinfo

import (
	"errors"
	"regexp"
	"runtime"
)

var Version = "dev"

const APISchemaVersion = "1"

type Info struct {
	GatewayVersion   string `json:"gateway_version"`
	APISchemaVersion string `json:"schema_version"`
	GoVersion        string `json:"go_version"`
}

func Current() Info { return Info{Version, APISchemaVersion, runtime.Version()} }
func ValidateReleaseVersion(v string) error {
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-beta[0-9]{3}$`).MatchString(v) || v[len(v)-3:] == "000" {
		return errors.New("invalid release version")
	}
	return nil
}
