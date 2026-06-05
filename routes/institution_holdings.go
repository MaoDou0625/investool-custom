package routes

import (
	"net/http"
	"strings"

	"github.com/axiaoxin-com/investool/core"
	secdata "github.com/axiaoxin-com/investool/datacenter/sec"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const defaultInstitution13FCacheFilename = "./institution_13f_cache.json"

type institutionHoldingsViewData struct {
	Env            string
	HostURL        string
	Version        string
	PageTitle      string
	Error          string
	Message        string
	Analysis       core.Institution13FAnalysis
	Profiles       []core.InstitutionProfile
	SelectedID     string
	CustomCIK      string
	CacheFile      string
	Refreshed      bool
	PortfolioError string
}

func InstitutionHoldingsPage(c *gin.Context) {
	profile := core.ResolveInstitutionProfile(c.Query("institution"), c.Query("cik"))
	forceRefresh := c.Query("refresh") == "1"
	portfolioContext := loadFundPortfolioAnalysisContextWithOptions(c, "", fundPortfolioAnalysisLoadOptions{
		UseLocalFundCache: true,
	})
	report, previous, refreshed, loadErr := loadInstitution13FReport(c, profile, forceRefresh)

	analysis := core.AnalyzeInstitution13F(report, previous, portfolioContext.ExposureReport, profile)
	pageErr := ""
	if loadErr != nil && len(report.Holdings) == 0 {
		pageErr = loadErr.Error()
	} else if loadErr != nil {
		analysis.Warnings = append(analysis.Warnings, loadErr.Error())
	}
	if portfolioContext.PageError != "" {
		analysis.Warnings = append(analysis.Warnings, "Portfolio exposure was not fully available: "+portfolioContext.PageError)
	}

	data := institutionHoldingsViewData{
		Env:            viper.GetString("env"),
		HostURL:        viper.GetString("server.host_url"),
		Version:        version.Version,
		PageTitle:      "InvesTool | 名人/机构持仓观察",
		Error:          pageErr,
		Message:        c.Query("message"),
		Analysis:       analysis,
		Profiles:       core.DefaultInstitutionProfiles(),
		SelectedID:     profile.ID,
		CustomCIK:      c.Query("cik"),
		CacheFile:      institution13FCacheFilename(),
		Refreshed:      refreshed,
		PortfolioError: portfolioContext.PageError,
	}
	c.HTML(http.StatusOK, "institution_holdings.html", data)
}

func loadInstitution13FReport(c *gin.Context, profile core.InstitutionProfile, forceRefresh bool) (models.Institution13FReport, *models.Institution13FReport, bool, error) {
	store := models.NewInstitution13FCacheStore(institution13FCacheFilename())
	if forceRefresh {
		client := secdata.NewClient(secdata.WithUserAgent(institution13FUserAgent()))
		current, previous, err := core.FetchInstitution13FReport(c, client, profile)
		if err != nil {
			cached, cachedPrevious, ok, cacheErr := loadInstitution13FFromCache(store, profile.ID)
			if cacheErr == nil && ok {
				return cached, cachedPrevious, false, err
			}
			return models.Institution13FReport{}, nil, false, err
		}
		if previous != nil {
			_ = store.UpsertReport(*previous, institution13FMaxReports())
		}
		if err := store.UpsertReport(current, institution13FMaxReports()); err != nil {
			return current, previous, true, err
		}
		return current, previous, true, nil
	}

	cached, previous, ok, err := loadInstitution13FFromCache(store, profile.ID)
	if err != nil {
		return models.Institution13FReport{}, nil, false, err
	}
	if ok {
		return cached, previous, false, nil
	}
	client := secdata.NewClient(secdata.WithUserAgent(institution13FUserAgent()))
	current, fetchedPrevious, err := core.FetchInstitution13FReport(c, client, profile)
	if err != nil {
		return models.Institution13FReport{}, nil, false, err
	}
	if fetchedPrevious != nil {
		_ = store.UpsertReport(*fetchedPrevious, institution13FMaxReports())
	}
	if err := store.UpsertReport(current, institution13FMaxReports()); err != nil {
		return current, fetchedPrevious, true, err
	}
	return current, fetchedPrevious, true, nil
}

func loadInstitution13FFromCache(store *models.Institution13FCacheStore, institutionID string) (models.Institution13FReport, *models.Institution13FReport, bool, error) {
	cache, err := store.Load()
	if err != nil {
		return models.Institution13FReport{}, nil, false, err
	}
	reports := cache.Reports[strings.TrimSpace(institutionID)]
	if len(reports) == 0 {
		return models.Institution13FReport{}, nil, false, nil
	}
	var previous *models.Institution13FReport
	if len(reports) > 1 {
		previous = &reports[1]
	}
	return reports[0], previous, true, nil
}

func institution13FCacheFilename() string {
	filename := viper.GetString("institution_13f.filename")
	if strings.TrimSpace(filename) == "" {
		return defaultInstitution13FCacheFilename
	}
	return filename
}

func institution13FUserAgent() string {
	userAgent := viper.GetString("institution_13f.sec_user_agent")
	if strings.TrimSpace(userAgent) == "" {
		return "MaoDou0625 investool-custom liuzi@local.invalid"
	}
	return userAgent
}

func institution13FMaxReports() int {
	maxReports := viper.GetInt("institution_13f.max_reports")
	if maxReports <= 0 {
		return 8
	}
	return maxReports
}
