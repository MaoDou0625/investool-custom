package models

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
)

// Fund4433RecommendationCache 持久化 4433 每日候选结果。
type Fund4433RecommendationCache struct {
	UpdatedAt   time.Time `json:"updated_at"`
	Source      string    `json:"source"`
	SourceCount int       `json:"source_count"`
	Items       FundList  `json:"items"`
}

// InitFund4433RecommendationList 从 json 文件加载 4433 每日候选列表。
func InitFund4433RecommendationList() error {
	content, err := ioutil.ReadFile(Fund4433RecommendationFilename)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	cache := Fund4433RecommendationCache{}
	if err := json.Unmarshal(content, &cache); err != nil {
		return err
	}
	ApplyFund4433RecommendationCache(cache)
	return nil
}

// ApplyFund4433RecommendationCache 将候选缓存应用到全局变量。
func ApplyFund4433RecommendationCache(cache Fund4433RecommendationCache) {
	Fund4433RecommendationList = cache.Items
	Fund4433RecommendationUpdatedAt = cache.UpdatedAt
	Fund4433RecommendationSource = cache.Source
	Fund4433RecommendationSourceCount = cache.SourceCount
}

// SaveFund4433RecommendationCache 保存并应用 4433 每日候选缓存。
func SaveFund4433RecommendationCache(cache Fund4433RecommendationCache) error {
	content, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(Fund4433RecommendationFilename, content, 0666); err != nil {
		return err
	}
	ApplyFund4433RecommendationCache(cache)
	return nil
}
