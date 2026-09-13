package tenant

import (
	"context"
	"sync/atomic"
)

type meterKey struct{}

// Meter follows one incoming request through settlement, including a charged
// upstream error. Retries and task completion do not create extra requests.
type Meter struct{ billed atomic.Bool }

func WithMeter(ctx context.Context) (context.Context, *Meter) {
	meter := &Meter{}
	return context.WithValue(ctx, meterKey{}, meter), meter
}

func (m *Meter) Billed() bool { return m.billed.Load() }

func MarkBilled(ctx context.Context) {
	if ctx == nil {
		return
	}
	if meter, ok := ctx.Value(meterKey{}).(*Meter); ok {
		meter.billed.Store(true)
	}
}
