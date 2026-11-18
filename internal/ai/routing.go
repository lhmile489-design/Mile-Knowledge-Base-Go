package ai

import "strings"

// RouteRule is one agent's trigger words and the reason shown to the user
// (shared/agentRouting.ts:37-119). Generic verbs ("帮我", "请") are
// deliberately absent: they appear in every request and would make the route
// random.
type RouteRule struct {
	Agent    string
	Keywords []string
	Because  string
}

// RouteRules migrates agentRouting.ts:44-119 verbatim: curator 21 words,
// scout 18, distiller 17, 56 in total. The order kept here is legacy order;
// the winner is picked by RoutePriority, not by this order.
var RouteRules = []RouteRule{
	{
		Agent: "curator",
		Keywords: []string{
			"整理", "归档", "分类", "归类", "移动", "重命名", "改名", "合并", "拆分", "清理",
			"重建索引", "建索引", "目录结构",
			"organize", "archive", "classify", "move", "rename", "merge", "reindex", "cleanup",
		},
		Because: "这是整理归档类请求",
	},
	{
		Agent: "scout",
		Keywords: []string{
			"搜索", "检索", "查找", "找一下", "找出", "在哪", "哪些文件", "有没有提到",
			"出处", "来源", "引用了", "相关的",
			"search", "find", "locate", "where is", "grep", "lookup",
		},
		Because: "这是检索类请求",
	},
	{
		Agent: "distiller",
		Keywords: []string{
			"总结", "摘要", "提炼", "归纳", "要点", "大纲", "草稿", "写一份", "整合成",
			"蒸馏", "读书笔记",
			"summarize", "summary", "outline", "distill", "draft", "takeaways",
		},
		Because: "这是提炼总结类请求",
	},
}

// RouteDecision is the route preview: who handles the text, the keywords that
// matched in word-list order, the reason fragment the UI shows, and whether
// the fallback agent was used.
type RouteDecision struct {
	Agent    string
	Matched  []string
	Because  string
	Fallback bool
}

// RouteToAgent routes one line of input to an agent. Pure, no IO
// (agentRouting.ts:138-189): lowercase containment matching, no
// tokenisation, word frequency never decides. When several rules hit, the
// lowest RoutePriority wins; a lone curator hit still routes to curator.
func RouteToAgent(text string) RouteDecision {
	if strings.TrimSpace(text) == "" {
		return RouteDecision{Agent: FallbackAgent, Because: "空输入", Fallback: true}
	}
	lower := strings.ToLower(text)
	var best *RouteDecision
	for index := range RouteRules {
		rule := &RouteRules[index]
		matched := make([]string, 0, 4)
		for _, keyword := range rule.Keywords {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				matched = append(matched, keyword)
			}
		}
		if len(matched) == 0 {
			continue
		}
		if best == nil || RoutePriority[rule.Agent] < RoutePriority[best.Agent] {
			best = &RouteDecision{Agent: rule.Agent, Matched: matched, Because: rule.Because}
		}
	}
	if best == nil {
		return RouteDecision{
			Agent:    FallbackAgent,
			Because:  "没有明确的整理/检索/提炼意图，按开放问答处理",
			Fallback: true,
		}
	}
	return *best
}

// ExplainRoute renders the full reason sentence the UI shows
// (agentRouting.ts:192-195). At most three keywords keep the line short; the
// fallback reason never fabricates one.
func ExplainRoute(decision RouteDecision, agentName string) string {
	if decision.Fallback {
		return decision.Because + " · 由" + agentName + "处理"
	}
	words := decision.Matched
	if len(words) > 3 {
		words = words[:3]
	}
	return decision.Because + "（识别到「" + strings.Join(words, "」「") + "」）· 由" + agentName + "处理"
}

// FollowUpRouteInput is one follow-up decision's inputs. Override is empty
// when the user has not picked an agent by hand.
type FollowUpRouteInput struct {
	Decision               RouteDecision
	Override               string
	CurrentTarget          string
	HasCurrentConversation bool
	RerouteRequested       bool
}

// FollowUpRoute is the resolved receiver and why it was chosen:
// auto | continue | override.
type FollowUpRoute struct {
	Agent string
	Kind  string
}

// ResolveFollowUpRoute keeps an ongoing conversation on its agent. Only two
// explicit actions move it: a manual override, or "自动" for the next message
// (followUpRouting.ts:34-40).
func ResolveFollowUpRoute(input FollowUpRouteInput) FollowUpRoute {
	if input.Override != "" {
		return FollowUpRoute{Agent: input.Override, Kind: "override"}
	}
	if input.RerouteRequested || !input.HasCurrentConversation {
		return FollowUpRoute{Agent: input.Decision.Agent, Kind: "auto"}
	}
	return FollowUpRoute{Agent: input.CurrentTarget, Kind: "continue"}
}
