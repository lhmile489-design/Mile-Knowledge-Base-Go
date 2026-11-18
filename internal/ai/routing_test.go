package ai

import (
	"strings"
	"testing"
)

func TestRouteIntent(t *testing.T) {
	curator := []string{
		"帮我把这些笔记整理一下",
		"把 raws 里的文件归档到 archives",
		"重命名这个文件",
		"please organize my notes",
		"重建索引",
	}
	for _, text := range curator {
		if got := RouteToAgent(text).Agent; got != "curator" {
			t.Errorf("%s -> %s", text, got)
		}
	}

	scout := []string{
		"搜索所有提到 CodeMirror 的文件",
		"这段话的出处在哪",
		"哪些文件引用了这篇",
		"find files about flexbox",
	}
	for _, text := range scout {
		if got := RouteToAgent(text).Agent; got != "scout" {
			t.Errorf("%s -> %s", text, got)
		}
	}

	distiller := []string{
		"总结一下这篇文章",
		"给我一份要点大纲",
		"把这几篇提炼成读书笔记",
		"summarize this document",
	}
	for _, text := range distiller {
		if got := RouteToAgent(text).Agent; got != "distiller" {
			t.Errorf("%s -> %s", text, got)
		}
	}
}

func TestRouteFallback(t *testing.T) {
	for _, text := range []string{"今天天气怎么样", "Electron 的 contextIsolation 是什么意思", "在吗"} {
		decision := RouteToAgent(text)
		if decision.Agent != FallbackAgent || !decision.Fallback || len(decision.Matched) != 0 {
			t.Errorf("%s -> %+v", text, decision)
		}
	}
}

func TestRouteEmptyInput(t *testing.T) {
	for _, text := range []string{"", "   "} {
		decision := RouteToAgent(text)
		if decision.Agent != FallbackAgent || !decision.Fallback || decision.Because != "空输入" {
			t.Errorf("%q -> %+v", text, decision)
		}
	}
}

func TestRouteCaseInsensitive(t *testing.T) {
	if got := RouteToAgent("SUMMARIZE this").Agent; got != "distiller" {
		t.Errorf("SUMMARIZE -> %s", got)
	}
	if got := RouteToAgent("Find the source").Agent; got != "scout" {
		t.Errorf("Find -> %s", got)
	}
}

func TestRoutePriority(t *testing.T) {
	if RoutePriority["scout"] != 0 || RoutePriority["general"] != 1 ||
		RoutePriority["distiller"] != 2 || RoutePriority["curator"] != 3 {
		t.Fatalf("priority table drifted: %v", RoutePriority)
	}
	cases := map[string]string{
		"找出所有草稿并整理归档": "scout",
		"查找并总结":       "scout",
		"总结并整理归档":     "distiller",
		"归档并总结":       "distiller",
		"搜索并总结":       "scout",
	}
	for text, want := range cases {
		if got := RouteToAgent(text).Agent; got != want {
			t.Errorf("%s -> %s, want %s", text, got, want)
		}
	}
}

func TestRouteOnlyExplicitCuratorIntent(t *testing.T) {
	for _, text := range []string{"这个文件讲了什么", "知识库里有多少内容", "解释一下这段代码"} {
		if got := RouteToAgent(text).Agent; got == "curator" {
			t.Errorf("%s -> curator", text)
		}
	}
}

func TestRouteAlwaysKnownAgent(t *testing.T) {
	samples := []string{"整理", "搜索", "总结", "随便说点什么", "", "整理并搜索并总结", strings.Repeat("a", 5000)}
	known := map[string]bool{"curator": true, "scout": true, "distiller": true, "general": true}
	for _, text := range samples {
		if got := RouteToAgent(text).Agent; !known[got] {
			t.Errorf("%q -> unknown agent %s", text, got)
		}
	}
}

func TestRouteWordLists(t *testing.T) {
	counts := map[string]int{"curator": 0, "scout": 0, "distiller": 0}
	total := 0
	for _, rule := range RouteRules {
		counts[rule.Agent] = len(rule.Keywords)
		total += len(rule.Keywords)
	}
	if counts["curator"] != 21 || counts["scout"] != 18 || counts["distiller"] != 17 || total != 56 {
		t.Fatalf("word lists drifted from agentRouting.ts:44-119: %v total=%d", counts, total)
	}
}

func TestExplainRoute(t *testing.T) {
	decision := RouteToAgent("帮我总结这篇")
	sentence := ExplainRoute(decision, "蒸馏学习")
	if !strings.Contains(sentence, "总结") || !strings.Contains(sentence, "蒸馏学习") {
		t.Fatalf("hit sentence: %s", sentence)
	}

	fallback := ExplainRoute(RouteToAgent("在吗"), "自由问答")
	if !strings.Contains(fallback, "自由问答") || strings.Contains(fallback, "「") {
		t.Fatalf("fallback sentence: %s", fallback)
	}

	many := ExplainRoute(RouteToAgent("整理 归档 分类 归类 移动 重命名"), "整理归档")
	if got := strings.Count(many, "「"); got > 3 {
		t.Fatalf("reason lists %d keywords: %s", got, many)
	}
}

func TestResolveFollowUpRoute(t *testing.T) {
	decision := RouteToAgent("搜索所有提到 flexbox 的文件")
	route := ResolveFollowUpRoute(FollowUpRouteInput{
		Decision: decision, CurrentTarget: FallbackAgent, HasCurrentConversation: false,
	})
	if route != (FollowUpRoute{Agent: "scout", Kind: "auto"}) {
		t.Fatalf("no conversation: %+v", route)
	}

	short := RouteToAgent("A")
	if short.Agent != FallbackAgent {
		t.Fatalf("short answer routed away: %+v", short)
	}
	route = ResolveFollowUpRoute(FollowUpRouteInput{
		Decision: short, CurrentTarget: "curator", HasCurrentConversation: true,
	})
	if route != (FollowUpRoute{Agent: "curator", Kind: "continue"}) {
		t.Fatalf("sticky conversation: %+v", route)
	}

	other := RouteToAgent("总结一下要点")
	if other.Agent != "distiller" {
		t.Fatalf("heading: %+v", other)
	}
	route = ResolveFollowUpRoute(FollowUpRouteInput{
		Decision: other, CurrentTarget: "curator", HasCurrentConversation: true,
	})
	if route != (FollowUpRoute{Agent: "curator", Kind: "continue"}) {
		t.Fatalf("intent switch inside a conversation: %+v", route)
	}

	route = ResolveFollowUpRoute(FollowUpRouteInput{
		Decision: other, CurrentTarget: "curator", HasCurrentConversation: true, RerouteRequested: true,
	})
	if route != (FollowUpRoute{Agent: "distiller", Kind: "auto"}) {
		t.Fatalf("explicit auto: %+v", route)
	}

	route = ResolveFollowUpRoute(FollowUpRouteInput{
		Decision: short, Override: "scout", CurrentTarget: "curator", HasCurrentConversation: true,
	})
	if route != (FollowUpRoute{Agent: "scout", Kind: "override"}) {
		t.Fatalf("override must win: %+v", route)
	}

	route = ResolveFollowUpRoute(FollowUpRouteInput{
		Decision: short, CurrentTarget: "curator", HasCurrentConversation: false, RerouteRequested: true,
	})
	if route != (FollowUpRoute{Agent: FallbackAgent, Kind: "auto"}) {
		t.Fatalf("auto with no conversation: %+v", route)
	}
}
