package core

import "testing"

func TestPreferClassCFundDailyActionsDropsAWhenBuyableCExists(t *testing.T) {
	actions := []FundDailyAction{
		{Code: "000001", Name: "测试成长混合A", Score: 95, SuggestedAmount: 100},
		{Code: "000002", Name: "测试成长混合C", Score: 80, SuggestedAmount: 100},
		{Code: "000003", Name: "其他稳健混合A", Score: 90, SuggestedAmount: 100},
	}

	got := preferClassCFundDailyActions(actions)
	if len(got) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(got))
	}
	if got[0].Code != "000002" {
		t.Fatalf("expected C class to remain first, got %s", got[0].Code)
	}
	if got[1].Code != "000003" {
		t.Fatalf("expected unrelated A class to remain, got %s", got[1].Code)
	}
	if len(got[0].Reasons) == 0 {
		t.Fatalf("expected C class reason, got %#v", got[0].Reasons)
	}
}

func TestPreferClassCFundDailyActionsKeepsAWhenCIsNotBuyable(t *testing.T) {
	actions := []FundDailyAction{
		{Code: "000001", Name: "测试成长混合A", Score: 95, SuggestedAmount: 100},
		{Code: "000002", Name: "测试成长混合C", Score: 80, SuggestedAmount: 0},
	}

	got := preferClassCFundDailyActions(actions)
	if len(got) != 2 {
		t.Fatalf("expected both classes to remain when C is not buyable, got %d", len(got))
	}
	if got[0].Code != "000001" || got[1].Code != "000002" {
		t.Fatalf("unexpected action order: %#v", got)
	}
}

func TestSplitFundShareClassHandlesLOFSuffix(t *testing.T) {
	baseA, classA := splitFundShareClass("融通人工智能指数(LOF)A")
	baseC, classC := splitFundShareClass("融通人工智能指数(LOF)C")
	if classA != fundShareClassA || classC != fundShareClassC {
		t.Fatalf("unexpected classes: %s %s", classA, classC)
	}
	if baseA != baseC {
		t.Fatalf("expected same base, got %s and %s", baseA, baseC)
	}
}

func TestSplitFundShareClassDoesNotTrimPlainEnglishWord(t *testing.T) {
	base, class := splitFundShareClass("ABC")
	if class != "" {
		t.Fatalf("expected no share class, got %s", class)
	}
	if base != "ABC" {
		t.Fatalf("unexpected base: %s", base)
	}
}
