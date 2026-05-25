package eastmoney

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFundHolderStructureResponse(t *testing.T) {
	raw := `var apidata={ content:"<table class='w782 comm cyrjg'><thead><tr><th class='first'>公告日期</th><th>机构持有比例</th><th>个人持有比例</th><th>内部持有比例</th><th class='last'>总份额（亿份）</th></tr></thead><tbody><tr><td>2025-12-31</td><td class='tor'>0.96%</td><td class='tor'>99.04%</td><td class='tor'>0.06%</td><td class='tor'>2.07</td></tr><tr><td>2025-06-30</td><td class='tor'>0.88%</td><td class='tor'>99.12%</td><td class='tor'>0.05%</td><td class='tor'>2.41</td></tr></tbody></table>",summary:"截至2025-12-31，景顺长城内需增长混合A 的基金机构持有0.02亿份，占总份额的0.96%"};`

	result, err := parseFundHolderStructureResponse(raw)

	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	require.NotNil(t, result.Latest)
	assert.Equal(t, "截至2025-12-31，景顺长城内需增长混合A 的基金机构持有0.02亿份，占总份额的0.96%", result.Summary)
	assert.Equal(t, "2025-12-31", result.Latest.AnnouncementDate)
	assert.Equal(t, 0.96, result.Latest.InstitutionalHoldingRatio)
	assert.Equal(t, 99.04, result.Latest.PersonalHoldingRatio)
	assert.Equal(t, 0.06, result.Latest.InternalHoldingRatio)
	assert.Equal(t, 2.07, result.Latest.TotalShare)
}

func TestParseFundHolderStructureResponseEmptyContent(t *testing.T) {
	raw := `var apidata={ content:"",summary:"暂无持有人结构数据"};`

	result, err := parseFundHolderStructureResponse(raw)

	require.NoError(t, err)
	assert.Equal(t, "暂无持有人结构数据", result.Summary)
	assert.Empty(t, result.Items)
	assert.Nil(t, result.Latest)
}
