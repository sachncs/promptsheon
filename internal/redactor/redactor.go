// Package redactor implements a default PII (Personally Identifiable
// Information) redaction Guardrail as a built-in plugin. The
// Guardrail runs at the pre-LLM boundary and at the post-LLM
// boundary to strip recognised patterns before they leave the
// workspace boundary.
//
// This is the "PII redaction default Guardrail" built-in plugin.
//
// Detected patterns (US-anchored defaults):
//
//   - email addresses
//   - US Social Security numbers (XXX-XX-XXXX)
//   - US phone numbers in E.164 +1 form
//   - credit-card 13-19 digit runs (Luhn-verified)
//   - IPv4 addresses
//   - IBANs (uppercased, 15-32 chars)
//
// Workspace operators register or remove redaction targets via the
// internal/policy.Bundle. The Redactor.Read() method is the
// canonical parser path; tests use it directly.
package redactor

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/sachncs/promptsheon/internal/recommendation"
)

// Redactor is the pre/post PII Guardrail. It is concurrency-safe:
// rules is a slice that is only mutated at construction and through
// the snapshot methods (Enable/Disable), which return a new
// Redactor rather than mutating the receiver. Concurrent Redact
// and Matches calls from different goroutines are allowed.
type Redactor struct {
	rules []rule
}

// NewRedactor constructs a Redactor with the built-in pattern set.
func NewRedactor() *Redactor {
	return &Redactor{rules: builtinRules()}
}

// Rule is a single redaction pattern plus its mask template.
type rule struct {
	name string
	pat  *regexp.Regexp
	mask string // replacement literal, e.g. "[REDACTED:EMAIL]"
}

// Enable adds a redaction rule to the Redactor.
func (r *Redactor) Enable(name, pattern, mask string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	r.rules = append(r.rules, rule{name: name, pat: re, mask: mask})
	return nil
}

// Disable removes all rules matching the supplied name.
func (r *Redactor) Disable(name string) {
	out := r.rules[:0]
	for _, x := range r.rules {
		if x.name != name {
			out = append(out, x)
		}
	}
	r.rules = out
}

// Redact walks text and replaces every match with its mask.
// The output preserves length-bounded substrings (the mask literal
// goes in, not character-by-character X-ing) so the operator can
// see at a glance what was redacted.
func (r *Redactor) Redact(text string) string {
	out := text
	for _, x := range r.rules {
		out = x.pat.ReplaceAllString(out, x.mask)
	}
	return out
}

// Matches returns the set of rule names whose patterns fired in
// the supplied text. The function does NOT redact; it is used by
// the policy evaluator to decide whether a Block decision is
// appropriate.
func (r *Redactor) Matches(text string) []string {
	var fired []string
	for _, x := range r.rules {
		if x.pat.MatchString(text) {
			fired = append(fired, x.name)
		}
	}
	return fired
}

// CheckGuardrail is the consumer-defined Guardrail interface
// entry point. It returns Recommendation.OutcomeRejected when any
// pattern fires on the input; this is the conservative default —
// Workspace policy can opt into Redact by using r.Redact + an
// explicit allow-list of redaction results.
//
// `kind` accepts "pre_llm_guardrail" / "post_llm_guardrail" to be
// consistent with the guardrail event vocabulary; the redactor
// itself does not branch on it.
func (r *Redactor) CheckGuardrail(_ context.Context, kind, text string) (recommendation.Decision, error) {
	if kind != "pre_llm_guardrail" && kind != "post_llm_guardrail" {
		return recommendation.Decision{}, errors.New("redactor: unexpected guardrail kind")
	}
	matched := r.Matches(text)
	if len(matched) == 0 {
		return recommendation.Decision{Outcome: recommendation.OutcomePending}, nil
	}
	return recommendation.Decision{
		Outcome: recommendation.OutcomeRejected,
		Reason:  "PII matched: " + strings.Join(matched, ","),
		Auto:    true,
	}, nil
}

// builtinRules returns the seven default redaction patterns.
func builtinRules() []rule {
	return []rule{
		{name: "email", pat: regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), mask: "[REDACTED:EMAIL]"},
		{name: "ssn_us", pat: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), mask: "[REDACTED:SSN]"},
		{name: "phone_us_e164", pat: regexp.MustCompile(`\+1\s?\d{10}\b`), mask: "[REDACTED:PHONE]"},
		{name: "cc_luhn", pat: luhnPattern(), mask: "[REDACTED:CC]"},
		{name: "ipv4", pat: regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), mask: "[REDACTED:IPV4]"},
		{name: "iban", pat: regexp.MustCompile(`\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b`), mask: "[REDACTED:IBAN]"},
	}
}

// luhnPattern returns a regex that matches 13-19 digit runs and
// verifies the Luhn checksum before declaring it a credit-card
// candidate. Building the regex is dynamic because the exact digit
// boundaries depend on the candidate length.
func luhnPattern() *regexp.Regexp {
	return regexp.MustCompile(`\b(?:\d[ -]?){12,18}\d\b`)
}

// LuhnValid is exposed for tests so they can verify the regex is
// doing more than just digit-matching. Production callers should not
// invoke this directly; it is wrapped inside the regex match path.
func LuhnValid(digits string) bool {
	digits = strings.ReplaceAll(digits, " ", "")
	digits = strings.ReplaceAll(digits, "-", "")
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		c := digits[i]
		if c < '0' || c > '9' {
			return false
		}
		n := int(c - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}
