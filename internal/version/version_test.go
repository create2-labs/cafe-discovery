package version

import (
	"os"
	"testing"
)

func TestCurrentPrefersAppVersionEnv(t *testing.T) {
	t.Setenv("APP_VERSION", "v9.9.9-env")
	t.Cleanup(func() { _ = os.Unsetenv("APP_VERSION") })

	if got := Current(); got != "v9.9.9-env" {
		t.Fatalf("Current() = %q, want v9.9.9-env", got)
	}
}

func TestPayloadShape(t *testing.T) {
	t.Setenv("APP_VERSION", "v1.0.0-test")
	t.Cleanup(func() { _ = os.Unsetenv("APP_VERSION") })

	payload := Payload()
	if payload.Version != "v1.0.0-test" {
		t.Fatalf("Payload().Version = %q, want v1.0.0-test", payload.Version)
	}
}
