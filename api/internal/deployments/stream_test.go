package deployments_test

import (
	"testing"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/deployments"
)

func TestHubFanout(t *testing.T) {
	h := deployments.NewHub()
	_, ch1, unsub1 := h.Subscribe("d1", 4)
	_, ch2, unsub2 := h.Subscribe("d1", 4)
	defer unsub1()
	defer unsub2()

	h.Publish("d1", deployments.Event{Type: deployments.EventLog, Text: "hi"})
	h.Publish("d2", deployments.Event{Type: deployments.EventLog, Text: "ignore"})
	for i, ch := range []<-chan deployments.Event{ch1, ch2} {
		select {
		case e := <-ch:
			if e.Text != "hi" {
				t.Fatalf("subscriber %d got %q", i, e.Text)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out", i)
		}
	}
}

func TestHubReplaysHistoryToLateSubscriber(t *testing.T) {
	h := deployments.NewHub()
	h.Publish("d1", deployments.Event{Type: deployments.EventStepStart, Step: "terraform.init"})
	h.Publish("d1", deployments.Event{Type: deployments.EventStepEnd, Step: "terraform.init", Status: "succeeded"})
	h.Publish("d1", deployments.Event{Type: deployments.EventLog, Text: "hello"})

	snap, _, unsub := h.Subscribe("d1", 4)
	defer unsub()

	if len(snap) != 3 {
		t.Fatalf("expected 3 history events, got %d", len(snap))
	}
	if snap[0].Step != "terraform.init" || snap[0].Type != deployments.EventStepStart {
		t.Fatalf("history[0] wrong: %+v", snap[0])
	}
	if snap[2].Type != deployments.EventLog || snap[2].Text != "hello" {
		t.Fatalf("history[2] wrong: %+v", snap[2])
	}
}
