package ansible_test

import (
	"strings"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/ansible"
)

func TestEventTeeWriterParsesNDJSON(t *testing.T) {
	var got []ansible.Event
	w := &ansible.TestEventWriter{}
	w.OnEvent = func(e ansible.Event) { got = append(got, e) }

	input := `{"uuid":"a","event":"playbook_on_start","event_data":{}}` + "\n" +
		`{"uuid":"b","event":"runner_on_ok","event_data":{"task":"foo"}}` + "\n" +
		"junk-no-json-line" + "\n"
	_, _ = w.Write([]byte(input))
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].Event != "playbook_on_start" || got[1].Event != "runner_on_ok" {
		t.Fatalf("events: %+v", got)
	}
	if task, ok := got[1].Data["task"].(string); !ok || !strings.HasPrefix(task, "foo") {
		t.Fatalf("task missing or not a string")
	}
}
