package explorer

type EtherscanTx struct {
	BlockNumber      string `json:"blockNumber"`
	TimeStamp        string `json:"timeStamp"`
	Hash             string `json:"hash"`
	Nonce            string `json:"nonce"`
	BlockHash        string `json:"blockHash"`
	TransactionIndex string `json:"transactionIndex"`
	From             string `json:"from"`
	To               string `json:"to"`
	Value            string `json:"value"`
	Input            string `json:"input"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	IsError          string `json:"isError"`
	TxReceiptStatus  string `json:"txreceipt_status"`
}

type EtherscanResp struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Result  []EtherscanTx `json:"result"`
}
