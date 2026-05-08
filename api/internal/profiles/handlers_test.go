package profiles_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/blueteam"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/greyspace"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/redteam"
)

func TestListProfilesReturnsFourEntries(t *testing.T) {
	reg := profiles.NewRegistry()
	reg.Register(redteam.New("", ""))
	reg.Register(blueteam.New("", ""))
	reg.Register(greyspace.New("", ""))
	reg.Register(profiles.NewStub(profiles.Metadata{Name: "homelab", Label: "Homelab", Enabled: true}))

	h := profiles.NewListHandler(reg)
	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	var got []profiles.Metadata
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 profiles got %d", len(got))
	}
	// Order is alphabetical by Name() — frontend depends on stable list order.
	expectedNames := []string{"blue-team", "grey-space", "homelab", "red-team"}
	for i, want := range expectedNames {
		if got[i].Name != want {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, want)
		}
	}
	// Spot-check that Metadata serializes through correctly.
	for _, p := range got {
		if p.Name == "homelab" && p.Label != "Homelab" {
			t.Errorf("homelab Label = %q, want %q", p.Label, "Homelab")
		}
	}
}
