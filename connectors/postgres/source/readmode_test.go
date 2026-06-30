package postgres

import "testing"

func TestChooseMode(t *testing.T) {
	tests := []struct {
		name     string
		corr     float64
		haveCorr bool
		sig      RouteSignals
		want     ReadMode
	}{
		{"high correlation → keyset", 0.95, true, RouteSignals{}, ModeKeyset},
		{"high negative correlation → keyset", -0.92, true, RouteSignals{}, ModeKeyset},
		{"low correlation, immutable pk → bitmap", 0.10, true, RouteSignals{}, ModeBitmap},
		{"no stats → treated as random → bitmap", 0, false, RouteSignals{}, ModeBitmap},
		{"low correlation, mutable pk → ctid+xmin", 0.10, true, RouteSignals{PKMutable: true}, ModeCtidXmin},
		{"append-only beats correlation → ctid append-only", 0.95, true, RouteSignals{AppendOnly: true}, ModeCtidAppendOnly},
		{"slot+cdc tops the hierarchy", 0.10, true, RouteSignals{SlotAvailable: true, CDC: true, PKMutable: true}, ModeSlot},
		{"slot without cdc does not trigger slot", 0.10, true, RouteSignals{SlotAvailable: true, PKMutable: true}, ModeCtidXmin},
		{"at threshold → keyset (inclusive)", correlationThreshold, true, RouteSignals{}, ModeKeyset},
		{"just below threshold → bitmap", correlationThreshold - 0.001, true, RouteSignals{}, ModeBitmap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chooseMode(tt.corr, tt.haveCorr, tt.sig); got != tt.want {
				t.Errorf("chooseMode(%v, %v, %+v) = %v, want %v", tt.corr, tt.haveCorr, tt.sig, got, tt.want)
			}
		})
	}
}
