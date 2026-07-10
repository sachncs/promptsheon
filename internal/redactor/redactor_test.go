package redactor

import (
	"context"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/recommendation"
)

func TestRedactEmail(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	out := r.Redact("contact alice@example.com for details")
	if !strings.Contains(out, "[REDACTED:EMAIL]") {
		t.Fatalf("expected email redacted, got %s", out)
	}
	if strings.Contains(out, "alice@example.com") {
		t.Fatalf("expected address removed, got %s", out)
	}
}

func TestRedactSSN(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	out := r.Redact("my SSN is 123-45-6789 do not share")
	if !strings.Contains(out, "[REDACTED:SSN]") {
		t.Fatalf("expected SSN redacted, got %s", out)
	}
}

func TestRedactPhoneE164(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	out := r.Redact("call +15551234567 today")
	if !strings.Contains(out, "[REDACTED:PHONE]") {
		t.Fatalf("expected phone redacted, got %s", out)
	}
}

func TestRedactValidCardLuhn(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	// 4111 1111 1111 1111 is the well-known Luhn-valid Visa test number.
	out := r.Redact("charge 4111111111111111 please")
	if !strings.Contains(out, "[REDACTED:CC]") {
		t.Fatalf("expected valid card redacted, got %s", out)
	}
}

func TestRedactNoMatch(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	in := "no PII here just numbers like 42"
	if out := r.Redact(in); out != in {
		t.Fatalf("expected unchanged, got %s", out)
	}
}

func TestMatchesList(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	got := r.Matches("email alice@example.com and SSN 123-45-6789")
	wantHas := func(name string) bool {
		for _, n := range got {
			if n == name {
				return true
			}
		}
		return false
	}
	if !wantHas("email") {
		t.Fatalf("expected email match")
	}
	if !wantHas("ssn_us") {
		t.Fatalf("expected ssn match")
	}
}

func TestEnableDisable(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	if err := r.Enable("custom", `SECRET`, "[REDACTED:CUSTOM]"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if out := r.Redact("SECRET and email bob@example.com"); !strings.Contains(out, "[REDACTED:CUSTOM]") {
		t.Fatalf("expected custom redaction, got %s", out)
	}
	r.Disable("custom")
	if out := r.Redact("SECRET and email bob@example.com"); strings.Contains(out, "[REDACTED:CUSTOM]") {
		t.Fatalf("expected custom gone, got %s", out)
	}
}

func TestCheckGuardrailBlocks(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	dec, err := r.CheckGuardrail(context.Background(), "pre_llm_guardrail", "alice@example.com called about job")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if dec.Outcome != recommendation.OutcomeRejected {
		t.Fatalf("expected rejected, got %s", dec.Outcome)
	}
}

func TestCheckGuardrailPassesCleanInput(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	dec, err := r.CheckGuardrail(context.Background(), "pre_llm_guardrail", "thanks for the help")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if dec.Outcome != recommendation.OutcomePending {
		t.Fatalf("expected pending, got %s", dec.Outcome)
	}
}

// Luhn test vectors.
func TestLuhnValidTrue(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"4111111111111111", "4242424242424242", "4012888888881881"} {
		if !LuhnValid(n) {
			t.Errorf("expected LuhnValid(%s) true", n)
		}
	}
}

func TestLuhnValidFalse(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"4111111111111112", "1234567890123"} {
		if LuhnValid(n) {
			t.Errorf("expected LuhnValid(%s) false", n)
		}
	}
}
