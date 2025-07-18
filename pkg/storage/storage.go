package storage

import (
	"fmt"
)

func AddPrice(symbol string, price float64, volume float64) {
	fmt.Printf("Added price for %s: %.2f with volume %.2f\n", symbol, price, volume)
}
