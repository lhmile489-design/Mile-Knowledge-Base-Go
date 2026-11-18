package ai

// RoutePriority orders the four agents when one input matches several rules:
// smaller wins, so "先检索" beats "顺手整理" (agentRouting.ts:17-26). Since the
// write boundary is uniform across agents, this only decides who handles the
// turn; it is not a permission ladder.
var RoutePriority = map[string]int{
	"scout":     0,
	"general":   1,
	"distiller": 2,
	"curator":   3,
}

// FallbackAgent handles open questions when no rule matches.
const FallbackAgent = "general"
