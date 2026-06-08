package sina

import "testing"

func TestParseSinaMarketQuoteLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		category string
		symbol   string
		quote    string
		pctSign  int
	}{
		{
			name:     "cn index",
			line:     `var hq_str_sh000001="上证指数,3938.7093,4027.7362,3959.3378,4007.4930,3927.8527,0,0,659293555,1267277235712,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2026-06-08,15:30:39,00,";`,
			category: MarketQuoteCategoryCNIndex,
			symbol:   "sh000001",
			quote:    "上证指数",
			pctSign:  -1,
		},
		{
			name:     "hk index",
			line:     `var hq_str_hkHSI="HSI,恒生指数,24582.910,24961.951,24837.980,24458.540,24657.059,-304.893,-1.221,0.00000,0.00000,363974709,18071003480,0.000,0.000,28056.100,23185.580,2026/06/08,16:09";`,
			category: MarketQuoteCategoryHKIndex,
			symbol:   "hkHSI",
			quote:    "恒生指数",
			pctSign:  -1,
		},
		{
			name:     "global index",
			line:     `var hq_str_b_NKY="日经225指数,64024.3800,-2563.74,-3.85,2:12 AM,14:12:00,2026-06-08,14:30:01,65947.5600,66588.1200,66115.1800,63406.6600,0";`,
			category: MarketQuoteCategoryGlobal,
			symbol:   "b_NKY",
			quote:    "日经225指数",
			pctSign:  -1,
		},
		{
			name:     "fx",
			line:     `var hq_str_fx_susdcnh="18:54:03,6.785800,6.785900,6.791000,95,6.790000,6.792100,6.782600,6.785800,离岸人民币（香港）,-0.080000,-0.005200,0.001399,,6.995700,6.758100,,2026-06-08";`,
			category: MarketQuoteCategoryFX,
			symbol:   "fx_susdcnh",
			quote:    "离岸人民币（香港）",
			pctSign:  -1,
		},
		{
			name:     "commodity",
			line:     `var hq_str_hf_CL="93.998,,93.860,93.870,95.470,92.200,18:53:18,90.540,93.000,0,1,3,2026-06-08,纽约原油,0";`,
			category: MarketQuoteCategoryCommodity,
			symbol:   "hf_CL",
			quote:    "纽约原油",
			pctSign:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, ok := parseSinaMarketQuoteLine(tt.line)
			if !ok {
				t.Fatal("expected quote")
			}
			if quote.Category != tt.category || quote.Symbol != tt.symbol || quote.Name != tt.quote {
				t.Fatalf("unexpected quote: %+v", quote)
			}
			if tt.pctSign < 0 && quote.ChangePercent >= 0 {
				t.Fatalf("expected negative change percent, got %.4f", quote.ChangePercent)
			}
			if tt.pctSign > 0 && quote.ChangePercent <= 0 {
				t.Fatalf("expected positive change percent, got %.4f", quote.ChangePercent)
			}
		})
	}
}
