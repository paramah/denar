package notification

import (
	"slices"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
)

// Preference is what a person wants for one event type in one household:
// a notification in the app or not, and the channels to send it to.
type Preference struct {
	Type       event.Type
	InApp      bool
	ChannelIDs []domain.ID
}

// DefaultPreference is the preference of a type nobody set: in the app,
// on no channel.
func DefaultPreference(t event.Type) Preference {
	return Preference{Type: t, InApp: true, ChannelIDs: []domain.ID{}}
}

// Merge returns a preference for every known event type, in the order of
// event.Types: the stored one where there is one, the default otherwise.
func Merge(stored []Preference) []Preference {
	out := make([]Preference, 0, len(event.Types))
	for _, t := range event.Types {
		p := DefaultPreference(t)
		if i := slices.IndexFunc(stored, func(s Preference) bool { return s.Type == t }); i >= 0 {
			p = stored[i]
			p.ChannelIDs = slices.Clone(p.ChannelIDs)
			if p.ChannelIDs == nil {
				p.ChannelIDs = []domain.ID{}
			}
		}
		out = append(out, p)
	}
	return out
}

// For returns the preference of the type among prefs, or the default.
func For(prefs []Preference, t event.Type) Preference {
	if i := slices.IndexFunc(prefs, func(p Preference) bool { return p.Type == t }); i >= 0 {
		return prefs[i]
	}
	return DefaultPreference(t)
}

// NormalizePreferences checks preferences given as input: known types,
// each once, channel ids without duplicates. It returns them sorted in the
// order of event.Types.
func NormalizePreferences(in []Preference) ([]Preference, error) {
	seen := map[event.Type]bool{}
	out := []Preference{}
	for _, p := range in {
		if !p.Type.Valid() {
			return nil, domain.ValidationError{Field: "items.type", Message: "is not a known event type: " + string(p.Type)}
		}
		if seen[p.Type] {
			return nil, domain.ValidationError{Field: "items.type", Message: "is given twice: " + string(p.Type)}
		}
		seen[p.Type] = true
		ids := []domain.ID{}
		for _, id := range p.ChannelIDs {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		if len(ids) > MaxChannels {
			return nil, domain.ValidationError{Field: "items.channel_ids", Message: "has too many channels"}
		}
		out = append(out, Preference{Type: p.Type, InApp: p.InApp, ChannelIDs: ids})
	}
	slices.SortFunc(out, func(a, b Preference) int {
		return slices.Index(event.Types, a.Type) - slices.Index(event.Types, b.Type)
	})
	return out, nil
}
