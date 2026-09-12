package confrepo

import (
	"fmt"
	"math"
	"strconv"
)

func ResolveTemperature(flagValue string, fileValue float32) (float32, error) {
	value := Resolve(flagValue, EnvNames("TEMPERATURE"), strconv.FormatFloat(float64(fileValue), 'g', -1, 32), "0")
	n, err := strconv.ParseFloat(value, 32)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("invalid temperature %q", value)
	}
	return float32(n), nil
}
