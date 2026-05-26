package routes

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/axiaoxin-com/investool/core"
	"github.com/gin-gonic/gin"
)

const maxFundPortfolioScreenshotSize = 10 << 20

func FundPortfolioRecognizeScreenshot(c *gin.Context) {
	header, err := requireFundPortfolioScreenshot(c)
	if err != nil {
		renderFundPortfolioPage(c, "", err.Error(), nil)
		return
	}

	tmpFilename, err := saveFundPortfolioScreenshotUpload(header)
	if err != nil {
		renderFundPortfolioPage(c, "", err.Error(), nil)
		return
	}
	defer os.Remove(tmpFilename)

	rawText, err := core.RecognizeImageText(c.Request.Context(), tmpFilename)
	if err != nil {
		renderFundPortfolioPage(c, "", err.Error(), nil)
		return
	}

	draft, err := core.ParseFundPortfolioScreenshotText(rawText)
	if err != nil {
		draft.RawText = rawText
		renderFundPortfolioPage(c, "", err.Error(), &draft)
		return
	}

	renderFundPortfolioPage(c, "截图识别完成，请核对下方预填字段后保存", "", &draft)
}

func requireFundPortfolioScreenshot(c *gin.Context) (*multipart.FileHeader, error) {
	header, err := c.FormFile("screenshot")
	if err != nil {
		return nil, fmt.Errorf("请选择支付宝基金持仓截图")
	}
	if header.Size <= 0 {
		return nil, fmt.Errorf("截图文件为空")
	}
	if header.Size > maxFundPortfolioScreenshotSize {
		return nil, fmt.Errorf("截图文件不能超过 10MB")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".bmp", ".gif", ".tif", ".tiff":
	default:
		return nil, fmt.Errorf("不支持的截图格式: %s", ext)
	}
	return header, nil
}

func saveFundPortfolioScreenshotUpload(header *multipart.FileHeader) (string, error) {
	src, err := header.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	tmp, err := os.CreateTemp("", "investool-fund-portfolio-*"+ext)
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, io.LimitReader(src, maxFundPortfolioScreenshotSize+1)); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	if info, err := tmp.Stat(); err == nil && info.Size() > maxFundPortfolioScreenshotSize {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("截图文件不能超过 10MB")
	}
	return tmp.Name(), nil
}
