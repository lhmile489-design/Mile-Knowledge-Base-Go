package mutation

import "github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"

// destructiveKinds need a pre-image (shared/mutation.ts DESTRUCTIVE_KINDS).
var destructiveKinds = map[string]bool{
	contract.MutEdit: true, contract.MutMove: true, contract.MutArchive: true,
	contract.MutTrash: true, contract.MutPermanentDelete: true,
}

// MaxPlanActions is the per-plan cap (shared/mutation.ts MAX_PLAN_ACTIONS).
const MaxPlanActions = 500

// AssessRisk is computed by Go and never taken from the caller
// (PRD FR-MUT, shared/mutation.ts assessRisk).
func AssessRisk(actions []contract.MutationAction) string {
	for _, action := range actions {
		if action.Kind == contract.MutPermanentDelete {
			return "high"
		}
	}
	if len(actions) > 20 {
		return "high"
	}
	for _, action := range actions {
		if destructiveKinds[action.Kind] {
			return "medium"
		}
		// importFile only replaces content when it pins an existing revision
		// (API §13.4); a pure create stays low.
		if action.Kind == contract.MutImportFile && action.ExpectedTargetRevision != nil {
			return "medium"
		}
	}
	return "low"
}

// isMutationKind lists the eight allowed kinds.
func isMutationKind(kind string) bool {
	switch kind {
	case contract.MutCreate, contract.MutCreateDirectory, contract.MutEdit,
		contract.MutMove, contract.MutCopy, contract.MutArchive,
		contract.MutTrash, contract.MutPermanentDelete, contract.MutImportFile:
		return true
	}
	return false
}
