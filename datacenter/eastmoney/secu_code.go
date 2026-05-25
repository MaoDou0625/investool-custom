package eastmoney

import "strings"

func normalizeSecurityCode(secuCode string) string {
	secuCode = strings.ToUpper(strings.TrimSpace(secuCode))
	secuCode = strings.TrimSuffix(secuCode, ".SH")
	secuCode = strings.TrimSuffix(secuCode, ".SZ")
	return secuCode
}

func formatPCSecuCode(secuCode string) string {
	upper := strings.ToUpper(strings.TrimSpace(secuCode))
	code := normalizeSecurityCode(upper)
	if strings.HasSuffix(upper, ".SH") {
		return "SH" + code
	}
	if strings.HasSuffix(upper, ".SZ") {
		return "SZ" + code
	}
	if strings.HasPrefix(code, "6") {
		return "SH" + code
	}
	return "SZ" + code
}
