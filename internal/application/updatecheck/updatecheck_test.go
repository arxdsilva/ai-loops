package updatecheck

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

type fakeSrc struct {
	v     string
	err   error
	calls int
}

func (f *fakeSrc) Latest(context.Context) (string, error) { f.calls++; return f.v, f.err }

type fakeStore struct {
	st      update.State
	loadErr error
	saveErr error
	saves   int
}

func (f *fakeStore) Load() (update.State, error) { return f.st, f.loadErr }
func (f *fakeStore) Save(s update.State) error   { f.saves++; f.st = s; return f.saveErr }

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

var now = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

func setup(st update.State, v string, err error) (*service, *fakeSrc, *fakeStore) {
	src, store := &fakeSrc{v: v, err: err}, &fakeStore{st: st}
	return New(src, store, fakeClock{now}), src, store
}

func TestCheckFetchesAndNotifiesOnce(t *testing.T) {
	svc, src, store := setup(update.State{}, "v0.4.0", nil)
	n, err := svc.Check(context.Background(), "0.3.0", true)
	if err != nil || n == nil || n.Latest != "v0.4.0" || n.Current != "0.3.0" {
		t.Fatalf("notice=%+v err=%v", n, err)
	}
	if src.calls != 1 || store.st.LastNotified != "v0.4.0" || !store.st.LastCheck.Equal(now) {
		t.Fatalf("calls=%d st=%+v", src.calls, store.st)
	}
	n, _ = svc.Check(context.Background(), "0.3.0", true)
	if n != nil || src.calls != 1 {
		t.Fatalf("second run: notice=%+v calls=%d", n, src.calls)
	}
}

func TestCheckPeekDoesNotMark(t *testing.T) {
	svc, _, store := setup(update.State{}, "v0.4.0", nil)
	n, _ := svc.Check(context.Background(), "0.3.0", false)
	if n == nil || store.st.LastNotified != "" {
		t.Fatalf("notice=%+v st=%+v", n, store.st)
	}
}

func TestCheckThrottled(t *testing.T) {
	st := update.State{LastCheck: now.Add(-time.Hour), Latest: "v0.4.0"}
	svc, src, _ := setup(st, "v9.9.9", nil)
	n, _ := svc.Check(context.Background(), "0.3.0", true)
	if src.calls != 0 || n == nil || n.Latest != "v0.4.0" {
		t.Fatalf("calls=%d notice=%+v", src.calls, n)
	}
}

func TestCheckFetchErrorSwallowed(t *testing.T) {
	st := update.State{LastCheck: now.Add(-48 * time.Hour), Latest: "v0.3.5", LastNotified: "v0.3.5"}
	svc, src, store := setup(st, "", errors.New("offline"))
	n, err := svc.Check(context.Background(), "0.3.0", true)
	if n != nil || err != nil {
		t.Fatalf("notice=%+v err=%v", n, err)
	}
	if !store.st.LastCheck.Equal(now) || store.st.Latest != "v0.3.5" || src.calls != 1 {
		t.Fatalf("st=%+v", store.st)
	}
	svc.Check(context.Background(), "0.3.0", true)
	if src.calls != 1 {
		t.Fatalf("offline re-probed: %d", src.calls)
	}
}

func TestCheckUpToDateAndLoadError(t *testing.T) {
	svc, _, store := setup(update.State{}, "v0.3.0", nil)
	store.loadErr = errors.New("corrupt")
	if n, _ := svc.Check(context.Background(), "0.3.0", true); n != nil {
		t.Fatalf("notice=%+v", n)
	}
}

func TestCheckSaveError(t *testing.T) {
	svc, _, store := setup(update.State{}, "v0.4.0", nil)
	store.saveErr = errors.New("ro")
	n, err := svc.Check(context.Background(), "0.3.0", true)
	if n == nil || err == nil {
		t.Fatalf("notice=%+v err=%v", n, err)
	}
}

func TestFetchIgnoresThrottle(t *testing.T) {
	st := update.State{LastCheck: now.Add(-time.Minute), Latest: "v0.3.0"}
	svc, src, store := setup(st, "v0.5.0", nil)
	got, err := svc.Fetch(context.Background())
	if err != nil || got.Latest != "v0.5.0" || src.calls != 1 || store.st.Latest != "v0.5.0" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	svc, _, store = setup(st, "", errors.New("x"))
	if _, err := svc.Fetch(context.Background()); err == nil || store.saves != 0 {
		t.Fatalf("err=%v saves=%d", err, store.saves)
	}
}
