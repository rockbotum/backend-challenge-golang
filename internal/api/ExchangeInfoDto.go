package api

type ExchangeInfoDto struct {
	rateLimits []RateLimit `json:"rateLimits"`
	symbols    []Symbol    `json:"symbols"`
}

type RateLimit struct {
	rateLimitType string `json:"rateLimitType"`
	interval      string `json:"interval"`
	intervalNum   int    `json:"intervalNum"`
	limit         int    `json:"limit"`
}

type Symbol struct {
	symbol     string `json:"symbol"`
	baseAsset  string `json:"baseAsset"`
	quoteAsset string `json:"quoteAsset"`
	status     string `json:"status"`
}
