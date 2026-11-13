package tracker

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type fakeLog struct {
	m       map[string][]run.Event
	created map[string]bool
}

func (f *fakeLog) Append(_ context.Context, id string, e run.Event) error {
	f.m[id] = append(f.m[id], e)
	return nil
}
func (f *fakeLog) Create(_ context.Context, id string) error {
	if _, ok := f.m[id]; ok || f.created[id] {
		return fs.ErrExist
	}
	f.created[id] = true
	return nil
}
func (f *fakeLog) Load(_ context.Context, id string) ([]run.Event, error) { return f.m[id], nil }
func (f *fakeLog) Dir(id string) string                                   { return "/d/" + id }

type fakePtr struct{ id string }

func (p *fakePtr) Current() (string, error)   { return p.id, nil }
func (p *fakePtr) SetCurrent(id string) error { p.id = id; return nil }
func (p *fakePtr) Clear() error               { p.id = ""; return nil }

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 11, 9, 15, 30, 5, 0, time.UTC) }

func newSvc() (*service, *fakeLog, *fakePtr) {
	l, p := &fakeLog{m: map[string][]run.Event{}, created: map[string]bool{}}, &fakePtr{}
	return New(l, p, fakeClock{}), l, p
}

func TestStartRecordEnd(t *testing.T) {
	ctx := context.Background()
	s, l, p := newSvc()
	if _, err := s.Current(ctx); !errors.Is(err, ErrNoActiveRun) {
		t.Fatal(err)
	}
	if err := s.Record(ctx, run.Event{Kind: run.KindNote}); !errors.Is(err, ErrNoActiveRun) {
		t.Fatal(err)
	}
	id, err := s.Start(ctx, StartInput{Title: "Add  Orders!! API", Ticket: "#1", Source: "github"})
	if err != nil || id != "20261109-1530-add-orders-api" || p.id != id {
		t.Fatalf("%q %v", id, err)
	}
	evs := l.m[id]
	if len(evs) != 2 || evs[0].Kind != run.KindRun || evs[0].Name != "Add  Orders!! API" || evs[0].Target != "#1" || evs[0].Note != "github" || evs[1].Phase != run.PhaseIntake {
		t.Fatalf("%+v", evs)
	}
	if err := s.Record(ctx, run.Event{Kind: run.KindNote, Note: "x"}); err != nil {
		t.Fatal(err)
	}
	e := l.m[id][2]
	if e.Run != id || e.Time.IsZero() {
		t.Fatalf("%+v", e)
	}
	st, err := s.State(ctx, "")
	if err != nil || st.ID != id || st.Phase != run.PhaseIntake {
		t.Fatalf("%+v %v", st, err)
	}
	if d, _ := s.Dir(ctx, ""); d != "/d/"+id {
		t.Fatal(d)
	}
	if evs, _ := s.Events(ctx, id); len(evs) != 3 {
		t.Fatal(len(evs))
	}
	if err := s.End(ctx, run.OutcomeDone); err != nil || p.id != "" {
		t.Fatal(err)
	}
	if l.m[id][3].Kind != run.KindEnd || l.m[id][3].Status != "done" {
		t.Fatalf("%+v", l.m[id][3])
	}
	if _, err := s.State(ctx, ""); !errors.Is(err, ErrNoActiveRun) {
		t.Fatal(err)
	}
	if _, err := s.Dir(ctx, ""); !errors.Is(err, ErrNoActiveRun) {
		t.Fatal(err)
	}
}

func TestStartCollision(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newSvc()
	a, _ := s.Start(ctx, StartInput{Title: "x"})
	b, _ := s.Start(ctx, StartInput{Title: "x"})
	c, _ := s.Start(ctx, StartInput{Title: "x"})
	if b != a+"-2" || c != a+"-3" {
		t.Fatal(a, b, c)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"":                        "run",
		"  --Hello, World--  ":    "hello-world",
		strings.Repeat("ab ", 30): "ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-a",
		"ÄÖ":                      "run",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("%q: %q want %q", in, got, want)
		}
		if len(slug(in)) > 40 {
			t.Error("long")
		}
	}
}

type failCreate struct{ fakeLog }

func (failCreate) Create(context.Context, string) error { return errors.New("disk full") }

func TestStartCreateError(t *testing.T) {
	l := &failCreate{fakeLog{m: map[string][]run.Event{}}}
	s := New(l, &fakePtr{}, fakeClock{})
	if _, err := s.Start(context.Background(), StartInput{Title: "x"}); err == nil {
		t.Fatal("want error")
	}
}
