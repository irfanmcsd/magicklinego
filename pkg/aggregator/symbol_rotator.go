package aggregator

type SymbolRotator struct {
	AllSymbols []string
	BatchSize  int
	CurrentIdx int
}

func NewSymbolRotator(symbols []string, batchSize int) *SymbolRotator {
	return &SymbolRotator{
		AllSymbols: symbols,
		BatchSize:  batchSize,
		CurrentIdx: 0,
	}
}

func (r *SymbolRotator) NextBatch() []string {
	n := len(r.AllSymbols)
	if n == 0 || r.BatchSize <= 0 {
		return nil
	}

	start := r.CurrentIdx
	end := start + r.BatchSize

	if end > n {
		end = n
	}

	batch := r.AllSymbols[start:end]

	// Move index for next round
	r.CurrentIdx = end % n

	return batch
}
