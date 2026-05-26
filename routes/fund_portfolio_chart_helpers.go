package routes

import "math"

func shortFundLabel(code string, name string) string {
	if name == "" {
		return code
	}
	runes := []rune(name)
	if len(runes) > 8 {
		name = string(runes[:8]) + "..."
	}
	return code + " " + name
}

func clampFloat(value float64, min float64, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}
