package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

type fakeFundDailySubscriptionInfoProvider struct {
	statuses map[string]string
}

func (p fakeFundDailySubscriptionInfoProvider) QueryFundInfo(ctx context.Context, fundCode string) (*eastmoney.RespFundInfo, error) {
	status, ok := p.statuses[fundCode]
	if !ok {
		return nil, fmt.Errorf("missing %s", fundCode)
	}
	resp := &eastmoney.RespFundInfo{}
	resp.Jjxq.Datas.Fcode = fundCode
	resp.Jjxq.Datas.Shortname = fundCode
	resp.Jjxq.Datas.Sgzt = status
	return resp, nil
}

func TestRefreshFundDailySubscriptionStatusesUpdatesSelectedFunds(t *testing.T) {
	candidate := &models.Fund{Code: "017093", Name: "stale candidate", SubscriptionStatus: "开放申购"}
	owned := &models.Fund{Code: "016556", Name: "owned fund"}
	provider := fakeFundDailySubscriptionInfoProvider{
		statuses: map[string]string{
			"017093": "暂停申购",
			"016556": "开放申购",
		},
	}

	warnings := refreshFundDailySubscriptionStatuses(context.Background(), provider, models.FundList{candidate}, []FundPortfolioAdvice{
		{Fund: owned},
	})

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if candidate.SubscriptionStatus != "暂停申购" {
		t.Fatalf("expected candidate status to refresh to suspended, got %q", candidate.SubscriptionStatus)
	}
	if candidate.CanSubscribe() {
		t.Fatalf("expected suspended candidate to be non-subscribable")
	}
	if owned.SubscriptionStatus != "开放申购" {
		t.Fatalf("expected owned status to refresh to open, got %q", owned.SubscriptionStatus)
	}
}

func TestRefreshFundDailySubscriptionStatusesFailsClosedWhenDetailMissing(t *testing.T) {
	candidate := &models.Fund{Code: "017093", Name: "stale candidate", SubscriptionStatus: "开放申购"}
	provider := fakeFundDailySubscriptionInfoProvider{statuses: map[string]string{}}

	warnings := refreshFundDailySubscriptionStatuses(context.Background(), provider, models.FundList{candidate}, nil)

	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %v", warnings)
	}
	if candidate.SubscriptionStatus != "未读取到申购状态" {
		t.Fatalf("expected candidate status to fail closed, got %q", candidate.SubscriptionStatus)
	}
	if candidate.CanSubscribe() {
		t.Fatalf("expected candidate with missing live status to be non-subscribable")
	}
}
