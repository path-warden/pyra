package changegate

import (
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/store"
)

// driftFindings reports, for each live Canon artifact, any symbol-id it cites
// whose path is a changed file but which no longer resolves in source (renamed,
// moved, or deleted). It reports the reference as unresolved rather than matching
// an incorrect symbol (REQ-505). It is skipped entirely when ops is nil, and a
// per-symbol resolution error never fails the run — it becomes a finding
// (REQ-205).
func driftFindings(s *store.Store, ops *codeintel.Ops, fileSet map[string]bool) []Finding {
	if s == nil || ops == nil {
		return nil
	}
	var out []Finding
	for i := range s.Canon {
		item := &s.Canon[i]
		if isDead(item.Status) {
			continue // a superseded/retired artifact's citations are moot
		}
		seen := map[string]bool{}
		for _, sid := range symbolIDRe.FindAllString(item.Body, -1) {
			if seen[sid] {
				continue
			}
			seen[sid] = true
			p, _, _, ok := codeintel.ParseID(sid)
			if !ok || !fileSet[p] {
				continue // only symbols whose file is in the change set
			}
			if _, err := ops.Source(sid, ""); err != nil {
				out = append(out, Finding{
					Code:     CodeSymbolUnresolved,
					File:     p,
					Artifact: item.ID,
					Type:     item.Type,
					Status:   item.Status,
					Title:    item.Title,
					Symbol:   sid,
				})
			}
		}
	}
	return out
}
