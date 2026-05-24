package pricing

// EstimateCost computes the maximum possible cost based on input tokens and max output tokens.
func EstimateCost(inputTokens, maxOutputTokens int, inputPricePer1M, outputPricePer1M float64) float64 {
	inputCost := (float64(inputTokens) * inputPricePer1M) / 1000000.0
	outputCost := (float64(maxOutputTokens) * outputPricePer1M) / 1000000.0
	return inputCost + outputCost
}

// ActualCost computes the actual cost based on input tokens and actual output tokens.
func ActualCost(inputTokens, outputTokens int, inputPricePer1M, outputPricePer1M float64) float64 {
	inputCost := (float64(inputTokens) * inputPricePer1M) / 1000000.0
	outputCost := (float64(outputTokens) * outputPricePer1M) / 1000000.0
	return inputCost + outputCost
}
