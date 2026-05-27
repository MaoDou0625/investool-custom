package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/gin-gonic/gin"
)

func FundPortfolioTianTianOpenDefaultBrowser(c *gin.Context) {
	if err := openURLInDefaultBrowser(tiantianFundHomeURL); err != nil {
		values := url.Values{}
		values.Set("error", fmt.Sprintf("无法启动默认浏览器: %v", err))
		c.Redirect(http.StatusFound, fundPortfolioBaseURL()+"/tiantian?"+values.Encode())
		return
	}
	c.Redirect(http.StatusFound, fundPortfolioBaseURL()+"/tiantian?opened=default")
}

func openURLInDefaultBrowser(target string) error {
	if _, err := url.ParseRequestURI(target); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
