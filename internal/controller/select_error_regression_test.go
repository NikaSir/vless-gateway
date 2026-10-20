package controller

import (
	"context"
	"reflect"
	"testing"

	"github.com/NikaSir/vless-gateway/internal/model"
)

type appliedSelectionError struct {
	*fakeClash
	err    error
	cancel context.CancelFunc
}

func (f *appliedSelectionError) Select(ctx context.Context, tag, route string) error {
	if err := f.fakeClash.Select(ctx, tag, route); err != nil {
		return err
	}
	if f.cancel != nil {
		f.cancel()
	}
	return f.err
}

func TestAppliedSelectionErrorIsConfirmedByLaterObservation(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{{"response-lost", context.DeadlineExceeded}, {"cancelled-after-application", context.Canceled}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeClash{selected: "direct"}
			s := newRegressionService(t, f, f, 3)
			s.PollOnce(context.Background())
			before, _ := s.PollOnce(context.Background())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			selector := &appliedSelectionError{fakeClash: f, err: tc.err}
			if tc.err == context.Canceled {
				selector.cancel = cancel
			}
			s.sel = selector
			if err := s.SetMode(ctx, model.ModePreferNetherlands); err == nil {
				t.Fatal("transport error accepted as confirmation")
			}
			if tc.err == context.Canceled && ctx.Err() != context.Canceled {
				t.Fatal("cancellation not exercised")
			}
			uncertain := s.Snapshot()
			if uncertain.ActiveRoute.Available || !reflect.DeepEqual(uncertain.LastChange, before.LastChange) || !reflect.DeepEqual(uncertain.History, before.History) {
				t.Fatal("attempt was published before observation")
			}
			after, err := s.PollOnce(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if after.ActiveRoute.Value == nil || *after.ActiveRoute.Value != model.RouteNetherlands || after.LastChange == nil || after.LastChange.Current != model.RouteNetherlands || after.LastChange.Previous != model.RouteGermany || after.LastChange.Trigger != "selector_confirmed" || !after.LastChange.At.After(after.ObservedAt) || len(after.History) != len(before.History)+1 {
				t.Fatalf("applied transition missing after acknowledgement: last=%+v history=%+v", after.LastChange, after.History)
			}
			again, err := s.PollOnce(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(again.LastChange, after.LastChange) || !reflect.DeepEqual(again.History, after.History) {
				t.Fatal("confirmed attempt recorded repeatedly")
			}
		})
	}
}
