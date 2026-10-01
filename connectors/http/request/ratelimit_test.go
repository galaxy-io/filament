package request

import (
	"testing"
	"time"
)

func TestLimiterBurstBudget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		limiter Limiter
		permits int
	}{
		{"default", NewStaticLimiter(30), 60},
		{"explicit", NewStaticLimiterWithBurst(30, 1), 1},
		{"dynamic", NewDynamicLimiter(30, DynamicConfig{Burst: 1}), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := tc.limiter
			if d, ok := l.(*dynamicLimiter); ok {
				l = d.base
			}
			bucket := l.(*staticLimiter).rl
			now := time.Now()
			for i := 0; i < tc.permits; i++ {
				if !bucket.AllowN(now, 1) {
					t.Fatalf("permit %d denied", i)
				}
			}
			if bucket.AllowN(now, 1) {
				t.Fatal("burst exceeded")
			}
		})
	}
}
