package notify

import (
	"testing"

	"github.com/shopspring/decimal"

	"prislage/internal/models"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func pdec(s string) *decimal.Decimal {
	d := dec(s)
	return &d
}

func TestDecide_TargetCross(t *testing.T) {
	// Cross target without qualifying as new_low (prev low already 900).
	types := Decide(pdec("1200"), pdec("900"), dec("999"), dec("1000"))
	if len(types) != 1 || types[0] != models.NotifyTargetReached {
		t.Fatalf("got %#v", types)
	}
}

func TestDecide_NoTargetSpam(t *testing.T) {
	types := Decide(pdec("950"), pdec("900"), dec("940"), dec("1000"))
	if len(types) != 0 {
		t.Fatalf("got %#v", types)
	}
}

func TestDecide_NewLowOnePercent(t *testing.T) {
	// prev low 1000, need <= 990
	types := Decide(pdec("1000"), pdec("1000"), dec("990"), dec("500"))
	if len(types) != 1 || types[0] != models.NotifyNewLow {
		t.Fatalf("got %#v", types)
	}
	types = Decide(pdec("1000"), pdec("1000"), dec("995"), dec("500"))
	if len(types) != 0 {
		t.Fatalf("expected no new_low for 0.5%% drop, got %#v", types)
	}
}
