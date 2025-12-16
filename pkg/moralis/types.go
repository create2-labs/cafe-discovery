package moralis

type MoralisTxResponse struct {
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Result   []MoralisTxResult `json:"result"`
}

type MoralisTxResult struct {
	Hash           string `json:"hash"`
	Nonce          string `json:"nonce"`
	BlockTimestamp string `json:"block_timestamp"`
	BlockNumber    string `json:"block_number"`
	BlockHash      string `json:"block_hash"`
}
