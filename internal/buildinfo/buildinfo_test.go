package buildinfo

import "testing"

func TestReleaseVersion(t *testing.T) {
	for _, v := range []string{"1.0.0-beta001", "2.13.4-beta999"} {
		if err := ValidateReleaseVersion(v); err != nil {
			t.Errorf("valid version rejected: %v", err)
		}
	}
	for _, v := range []string{"v1.0.0-beta001", "1.0.0", "1.0.0-beta1", "1.0.0-beta000", "1.0.0-beta001\n", "01.0.0-beta001"} {
		if ValidateReleaseVersion(v) == nil {
			t.Error("invalid release version accepted")
		}
	}
}
func TestInjectedVersion(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	Version = "1.0.0-beta001"
	got := Current()
	if got.GatewayVersion != "1.0.0-beta001" || got.APISchemaVersion != "1" || got.GoVersion == "" {
		t.Fatalf("bad build metadata: %+v", got)
	}
}
