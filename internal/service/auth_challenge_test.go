package service

import (
	"errors"
	"testing"

	"cafe-discovery/internal/config"

	"github.com/spf13/viper"
)

// challengeDevValue is the provider's published development value, an
// acceptable literal under cafe-secrets section 1.
const challengeDevValue = "1x0000000000000000000000000000000AA"

// setChallengeSecret sets the challenge secret through viper and restores the
// previous value when the test ends.
func setChallengeSecret(t *testing.T, value string) {
	t.Helper()
	previous := viper.Get(config.TurnstileSecretKey)
	viper.Set(config.TurnstileSecretKey, value)
	t.Cleanup(func() { viper.Set(config.TurnstileSecretKey, previous) })
}

// CA OC-P99: Discovery refuses to start without the challenge secret.
func TestCheckSignInChallengeConfig(t *testing.T) {
	t.Run("unset is refused with the fixed message", func(t *testing.T) {
		setChallengeSecret(t, "")
		err := CheckSignInChallengeConfig()
		if err == nil {
			t.Fatal("expected an error when the secret is unset")
		}
		want := config.TurnstileSecretKey + " not set in config file or environment variable"
		if err.Error() != want {
			t.Fatalf("error text = %q, want %q", err.Error(), want)
		}
	})

	t.Run("development value is accepted", func(t *testing.T) {
		setChallengeSecret(t, challengeDevValue)
		if err := CheckSignInChallengeConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("any other non-empty value is accepted", func(t *testing.T) {
		setChallengeSecret(t, "changeme-test")
		if err := CheckSignInChallengeConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// CA OC-P99: a caller that skips the startup check still fails closed.
func TestChallengeVerificationFailsClosedWhenUnset(t *testing.T) {
	setChallengeSecret(t, "")
	err := verifyTurnstileToken("some-token")
	if !errors.Is(err, ErrTurnstileValidation) {
		t.Fatalf("error = %v, want ErrTurnstileValidation", err)
	}
}

// CA OC-P99: the development-value behaviour is kept.
func TestChallengeVerificationDevelopmentValue(t *testing.T) {
	setChallengeSecret(t, challengeDevValue)

	if err := verifyTurnstileToken("some-token"); err != nil {
		t.Fatalf("non-empty token refused: %v", err)
	}
	if err := verifyTurnstileToken(""); !errors.Is(err, ErrTurnstileValidation) {
		t.Fatalf("empty token error = %v, want ErrTurnstileValidation", err)
	}
}
