package sec

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultSECUserAgent      = "MaoDou0625 investool-custom liuzi@local.invalid"
	defaultSECDataBaseURL    = "https://data.sec.gov"
	defaultSECArchiveBaseURL = "https://www.sec.gov"
)

type Client struct {
	httpClient     *http.Client
	userAgent      string
	dataBaseURL    string
	archiveBaseURL string
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if strings.TrimSpace(userAgent) != "" {
			c.userAgent = strings.TrimSpace(userAgent)
		}
	}
}

func WithBaseURLs(dataBaseURL, archiveBaseURL string) Option {
	return func(c *Client) {
		if strings.TrimSpace(dataBaseURL) != "" {
			c.dataBaseURL = strings.TrimRight(strings.TrimSpace(dataBaseURL), "/")
		}
		if strings.TrimSpace(archiveBaseURL) != "" {
			c.archiveBaseURL = strings.TrimRight(strings.TrimSpace(archiveBaseURL), "/")
		}
	}
}

func NewClient(options ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		userAgent:      defaultSECUserAgent,
		dataBaseURL:    defaultSECDataBaseURL,
		archiveBaseURL: defaultSECArchiveBaseURL,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type Filing struct {
	CIK             string
	InstitutionName string
	Form            string
	AccessionNumber string
	FilingDate      string
	ReportDate      string
	PrimaryDocument string
}

type InformationTableEntry struct {
	NameOfIssuer         string
	TitleOfClass         string
	CUSIP                string
	ValueThousands       float64
	SharesOrPrincipal    float64
	ShareType            string
	PutCall              string
	InvestmentDiscretion string
}

type Report struct {
	CIK                 string
	InstitutionName     string
	Filing              Filing
	InformationTableURL string
	Entries             []InformationTableEntry
}

type submissionsResponse struct {
	CIK     string `json:"cik"`
	Name    string `json:"name"`
	Filings struct {
		Recent struct {
			AccessionNumber []string `json:"accessionNumber"`
			FilingDate      []string `json:"filingDate"`
			ReportDate      []string `json:"reportDate"`
			Form            []string `json:"form"`
			PrimaryDocument []string `json:"primaryDocument"`
		} `json:"recent"`
	} `json:"filings"`
}

type archiveIndexResponse struct {
	Directory struct {
		Name string             `json:"name"`
		Item []archiveIndexItem `json:"item"`
	} `json:"directory"`
}

type archiveIndexItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size string `json:"size"`
}

type informationTableXML struct {
	Entries []informationTableEntryXML `xml:"infoTable"`
}

type informationTableEntryXML struct {
	NameOfIssuer         string `xml:"nameOfIssuer"`
	TitleOfClass         string `xml:"titleOfClass"`
	CUSIP                string `xml:"cusip"`
	Value                string `xml:"value"`
	SSHPrnamtType        string `xml:"shrsOrPrnAmt>sshPrnamtType"`
	SSHPrnamt            string `xml:"shrsOrPrnAmt>sshPrnamt"`
	PutCall              string `xml:"putCall"`
	InvestmentDiscretion string `xml:"investmentDiscretion"`
}

func (c *Client) FetchLatest13FReports(ctx context.Context, cik string, count int) ([]Report, error) {
	if count <= 0 {
		count = 1
	}
	filings, err := c.Latest13FFilings(ctx, cik, count)
	if err != nil {
		return nil, err
	}
	reports := make([]Report, 0, len(filings))
	for _, filing := range filings {
		report, err := c.Fetch13FReport(ctx, filing)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (c *Client) Latest13FFilings(ctx context.Context, cik string, count int) ([]Filing, error) {
	normalizedCIK := normalizeCIK(cik)
	if normalizedCIK == "" {
		return nil, fmt.Errorf("empty SEC CIK")
	}

	submissionsURL := fmt.Sprintf("%s/submissions/CIK%s.json", c.dataBaseURL, normalizedCIK)
	var submissions submissionsResponse
	if err := c.getJSON(ctx, submissionsURL, &submissions); err != nil {
		return nil, err
	}

	filings := []Filing{}
	recent := submissions.Filings.Recent
	for idx, form := range recent.Form {
		if form != "13F-HR" && form != "13F-HR/A" {
			continue
		}
		filing := Filing{
			CIK:             normalizedCIK,
			InstitutionName: submissions.Name,
			Form:            form,
			AccessionNumber: atString(recent.AccessionNumber, idx),
			FilingDate:      atString(recent.FilingDate, idx),
			ReportDate:      atString(recent.ReportDate, idx),
			PrimaryDocument: atString(recent.PrimaryDocument, idx),
		}
		if filing.AccessionNumber == "" {
			continue
		}
		filings = append(filings, filing)
		if len(filings) >= count {
			break
		}
	}
	if len(filings) == 0 {
		return nil, fmt.Errorf("no 13F-HR filing found for CIK %s", normalizedCIK)
	}
	return filings, nil
}

func (c *Client) Fetch13FReport(ctx context.Context, filing Filing) (Report, error) {
	baseURL := c.filingBaseURL(filing.CIK, filing.AccessionNumber)
	infoTableURL, entries, err := c.findInformationTable(ctx, baseURL)
	if err != nil {
		return Report{}, err
	}
	return Report{
		CIK:                 normalizeCIK(filing.CIK),
		InstitutionName:     filing.InstitutionName,
		Filing:              filing,
		InformationTableURL: infoTableURL,
		Entries:             entries,
	}, nil
}

func (c *Client) filingBaseURL(cik string, accessionNumber string) string {
	cikPath := strings.TrimLeft(normalizeCIK(cik), "0")
	if cikPath == "" {
		cikPath = "0"
	}
	accessionPath := strings.ReplaceAll(accessionNumber, "-", "")
	return fmt.Sprintf("%s/Archives/edgar/data/%s/%s", c.archiveBaseURL, cikPath, accessionPath)
}

func (c *Client) findInformationTable(ctx context.Context, baseURL string) (string, []InformationTableEntry, error) {
	candidates, err := c.listArchiveXMLCandidates(ctx, baseURL, 0)
	if err != nil {
		return "", nil, err
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return informationTableCandidateScore(candidates[i]) > informationTableCandidateScore(candidates[j])
	})
	for _, candidate := range candidates {
		body, err := c.getBytes(ctx, candidate)
		if err != nil {
			continue
		}
		entries, err := ParseInformationTableXML(body)
		if err == nil && len(entries) > 0 {
			return candidate, entries, nil
		}
	}
	return "", nil, fmt.Errorf("no readable 13F information table XML found under %s", baseURL)
}

func (c *Client) listArchiveXMLCandidates(ctx context.Context, baseURL string, depth int) ([]string, error) {
	indexURL := strings.TrimRight(baseURL, "/") + "/index.json"
	var index archiveIndexResponse
	if err := c.getJSON(ctx, indexURL, &index); err != nil {
		return nil, err
	}
	candidates := []string{}
	for _, item := range index.Directory.Item {
		name := strings.TrimSpace(item.Name)
		if name == "" || name == "." || name == ".." {
			continue
		}
		itemURL := strings.TrimRight(baseURL, "/") + "/" + url.PathEscape(name)
		lowerName := strings.ToLower(name)
		if strings.EqualFold(item.Type, "dir") && depth < 1 {
			nested, err := c.listArchiveXMLCandidates(ctx, itemURL, depth+1)
			if err == nil {
				candidates = append(candidates, nested...)
			}
			continue
		}
		if strings.HasSuffix(lowerName, ".xml") {
			candidates = append(candidates, itemURL)
		}
	}
	return candidates, nil
}

func ParseInformationTableXML(content []byte) ([]InformationTableEntry, error) {
	table := informationTableXML{}
	if err := xml.Unmarshal(content, &table); err != nil {
		return nil, err
	}
	entries := make([]InformationTableEntry, 0, len(table.Entries))
	for _, item := range table.Entries {
		entry := InformationTableEntry{
			NameOfIssuer:         strings.TrimSpace(item.NameOfIssuer),
			TitleOfClass:         strings.TrimSpace(item.TitleOfClass),
			CUSIP:                strings.ToUpper(strings.TrimSpace(item.CUSIP)),
			ValueThousands:       parseSECFloat(item.Value),
			SharesOrPrincipal:    parseSECFloat(item.SSHPrnamt),
			ShareType:            strings.TrimSpace(item.SSHPrnamtType),
			PutCall:              strings.TrimSpace(item.PutCall),
			InvestmentDiscretion: strings.TrimSpace(item.InvestmentDiscretion),
		}
		if entry.NameOfIssuer == "" && entry.CUSIP == "" {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (c *Client) getJSON(ctx context.Context, target string, out interface{}) error {
	body, err := c.getBytes(ctx, target)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *Client) getBytes(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json, text/xml, application/xml, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("SEC request %s returned %s", target, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func normalizeCIK(cik string) string {
	cleaned := strings.TrimSpace(cik)
	cleaned = strings.TrimPrefix(strings.ToUpper(cleaned), "CIK")
	cleaned = strings.TrimLeft(cleaned, "0")
	if cleaned == "" {
		return ""
	}
	if len(cleaned) >= 10 {
		return cleaned
	}
	return strings.Repeat("0", 10-len(cleaned)) + cleaned
}

func atString(values []string, idx int) string {
	if idx < 0 || idx >= len(values) {
		return ""
	}
	return values[idx]
}

func parseSECFloat(value string) float64 {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0
	}
	var result float64
	_, _ = fmt.Sscanf(value, "%f", &result)
	return result
}

func informationTableCandidateScore(candidate string) int {
	lower := strings.ToLower(candidate)
	score := 0
	for _, keyword := range []string{"infotable", "informationtable", "form13f", "xslform13f"} {
		if strings.Contains(lower, keyword) {
			score += 10
		}
	}
	if strings.Contains(lower, "primary_doc") {
		score -= 20
	}
	return score
}
