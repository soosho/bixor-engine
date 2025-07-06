package models

import (
	"github.com/shopspring/decimal"
)

// DecimalFromString creates a decimal from string with error handling
func DecimalFromString(value string) decimal.Decimal {
	d, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// DecimalFromFloat creates a decimal from float64
func DecimalFromFloat(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value)
}

// DecimalFromInt creates a decimal from int64
func DecimalFromInt(value int64) decimal.Decimal {
	return decimal.NewFromInt(value)
} 