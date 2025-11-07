package messages

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/shruggr/inspiration/kvstore"
)

// FetchSubtreeData fetches subtree data from a URL and parses the transactions
// Returns SubtreeData with verified transaction IDs (hashed locally)
func FetchSubtreeData(ctx context.Context, url string, merkleRoot kvstore.Hash) (*SubtreeData, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subtree data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Parse transactions from the response body
	var txIDs []kvstore.Hash
	var txBytes [][]byte

	for {
		tx := &transaction.Transaction{}
		_, err := tx.ReadFrom(resp.Body)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse transaction: %w", err)
		}

		// Serialize and hash the transaction ourselves (don't trust external data)
		txData := tx.Bytes()

		// Calculate txid by hashing the transaction
		txid := tx.TxID()
		if txid == nil {
			return nil, fmt.Errorf("failed to calculate txid")
		}

		txIDs = append(txIDs, *txid)
		txBytes = append(txBytes, txData)
	}

	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no transactions found in subtree data")
	}

	return &SubtreeData{
		MerkleRoot: merkleRoot,
		TxIDs:      txIDs,
		Txs:        txBytes,
	}, nil
}
