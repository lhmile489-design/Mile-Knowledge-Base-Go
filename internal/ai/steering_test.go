package ai

import (
	"strings"
	"testing"
)

func TestRenderSteeringBlock(t *testing.T) {
	block := RenderSteeringBlock("优先使用中文回答")
	if !strings.HasPrefix(block, SteeringBlockHeading) {
		t.Fatalf("heading: %q", block)
	}
	preambleAt := strings.Index(block, SteeringPreamble)
	steeringAt := strings.Index(block, "优先使用中文回答")
	if preambleAt < 0 || steeringAt < 0 || preambleAt > steeringAt {
		t.Fatalf("the permission preamble must come before the user text: %q", block)
	}
	if !strings.Contains(block, "无法扩大") {
		t.Fatalf("preamble lost: %q", block)
	}
}
