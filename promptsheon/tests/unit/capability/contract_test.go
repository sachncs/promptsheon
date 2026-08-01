//go:build tests_migration


package capability_test

import (
	"errors"
	"testing"

	. "github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/errs"
)

func TestContractValid(t *testing.T) {
	t.Parallel()
	c := CapabilityContract{
		BlastRadius:    BlastLow,
		AutoPromotable: true,
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid contract: %v", err)
	}
}

func TestContractEmptyIsError(t *testing.T) {
	t.Parallel()
	if err := (CapabilityContract{}).Validate(); !errors.Is(err, errs.ErrEmptyContract) {
		t.Fatalf("empty contract must return errs.ErrEmptyContract, got %v", err)
	}
}

func TestContractInvalidBlastRadius(t *testing.T) {
	t.Parallel()
	c := CapabilityContract{BlastRadius: "catastrophic"}
	if err := c.Validate(); !errors.Is(err, errs.ErrInvalidBlastRadius) {
		t.Fatalf("invalid blast radius must return errs.ErrInvalidBlastRadius, got %v", err)
	}
}

func TestContractSuccessRateRange(t *testing.T) {
	t.Parallel()
	for _, rate := range []float64{-0.1, 1.1, 2.0} {
		c := CapabilityContract{BlastRadius: BlastLow, SLOTarget: SLOTarget{MinSuccessRate: rate}}
		if err := c.Validate(); err == nil {
			t.Errorf("success rate %f must fail validation", rate)
		}
	}
}

func TestContractHallucinationRateRange(t *testing.T) {
	t.Parallel()
	for _, rate := range []float64{-0.1, 1.1, 2.0} {
		c := CapabilityContract{BlastRadius: BlastLow, SLOTarget: SLOTarget{MaxHallucinationRate: rate}}
		if err := c.Validate(); err == nil {
			t.Errorf("hallucination rate %f must fail validation", rate)
		}
	}
}

func TestContractCanAutoAdoptLow(t *testing.T) {
	t.Parallel()
	c := CapabilityContract{
		BlastRadius: BlastLow,
		SLOTarget:   SLOTarget{MaxP95LatencyMS: 1000},
	}
	if !c.CanAutoAdopt() {
		t.Error("low blast radius with a valid SLO must be auto-adoptable")
	}
	cNoSLO := CapabilityContract{BlastRadius: BlastLow}
	if cNoSLO.CanAutoAdopt() {
		t.Error("low blast radius without an SLO must NOT auto-adopt")
	}
}

func TestContractCanAutoAdoptMediumRequiresOpt(t *testing.T) {
	t.Parallel()
	cOff := CapabilityContract{BlastRadius: BlastMedium, SLOTarget: SLOTarget{MaxP95LatencyMS: 1000}}
	if cOff.CanAutoAdopt() {
		t.Error("medium blast radius without AutoPromotable must not auto-adopt")
	}
	cOn := CapabilityContract{
		BlastRadius:    BlastMedium,
		AutoPromotable: true,
		SLOTarget:      SLOTarget{MaxP95LatencyMS: 1000},
	}
	if !cOn.CanAutoAdopt() {
		t.Error("medium blast radius with AutoPromotable and an SLO must auto-adopt")
	}
}

func TestContractCanAutoAdoptHighRequiresOpt(t *testing.T) {
	t.Parallel()
	cOff := CapabilityContract{BlastRadius: BlastHigh, SLOTarget: SLOTarget{MaxP95LatencyMS: 1000}}
	if cOff.CanAutoAdopt() {
		t.Error("high blast radius without AutoPromotable must not auto-adopt")
	}
	cOn := CapabilityContract{
		BlastRadius:    BlastHigh,
		AutoPromotable: true,
		SLOTarget:      SLOTarget{MaxP95LatencyMS: 1000},
	}
	if !cOn.CanAutoAdopt() {
		t.Error("high blast radius with AutoPromotable and an SLO must auto-adopt")
	}
}

func TestContractCanAutoAdoptInvalidIsFalse(t *testing.T) {
	t.Parallel()
	c := CapabilityContract{
		BlastRadius: "wat",
		SLOTarget:   SLOTarget{MaxP95LatencyMS: 1000},
	}
	if c.CanAutoAdopt() {
		t.Error("invalid contract must never auto-adopt")
	}
}

func TestBlastRadiusValid(t *testing.T) {
	t.Parallel()
	for _, b := range []BlastRadius{BlastLow, BlastMedium, BlastHigh} {
		if !b.Valid() {
			t.Errorf("%q must be valid", b)
		}
	}
	for _, b := range []BlastRadius{"", "critical", "low-medium"} {
		if b.Valid() {
			t.Errorf("%q must be invalid", b)
		}
	}
}
