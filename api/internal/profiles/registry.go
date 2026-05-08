package profiles

import (
	"fmt"
	"sort"
)

type Registry struct {
	profiles map[string]Profile
}

func NewRegistry() *Registry { return &Registry{profiles: map[string]Profile{}} }

func (r *Registry) Register(p Profile) { r.profiles[p.Name()] = p }

func (r *Registry) Get(name string) (Profile, error) {
	p, ok := r.profiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown profile: %s", name)
	}
	return p, nil
}

// All returns all registered profiles, sorted by Name() for deterministic output.
func (r *Registry) All() []Profile {
	out := make([]Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}
