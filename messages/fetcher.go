package messages

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/shruggr/inspiration/kvstore"
)

// FetchSubtreeTxIDs fetches the list of transaction IDs from a subtree (Format 1)
// Endpoint: GET {baseURL}/api/v1/subtree/{subtreeHash}
// Returns a stream of 32-byte transaction IDs concatenated together
func FetchSubtreeTxIDs(ctx context.Context, baseURL string, subtreeHash kvstore.Hash) ([]kvstore.Hash, error) {
	url := fmt.Sprintf("%s/api/v1/subtree/%s", baseURL, chainhash.Hash(subtreeHash).String())

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subtree txids: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("subtree not found: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	var txIDs []kvstore.Hash
	buffer := make([]byte, 32)

	for {
		n, err := io.ReadFull(resp.Body, buffer)
		if err == io.EOF {
			break
		}
		if err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("partial txid read, expected 32 bytes got %d", n)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read txid: %w", err)
		}

		var txid kvstore.Hash
		copy(txid[:], buffer)
		txIDs = append(txIDs, txid)
	}

	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no transaction IDs found in subtree")
	}

	return txIDs, nil
}

// FetchTransactionsByTxID fetches specific transactions by txid (Format 2a - general)
// Endpoint: POST {baseURL}/api/v1/txs
// Request body: concatenated 32-byte transaction IDs to fetch
// Returns the full transaction data for the requested txids
// Use this when fetching a small number of specific transactions
func FetchTransactionsByTxID(ctx context.Context, baseURL string, txIDs []kvstore.Hash) ([][]byte, error) {
	if len(txIDs) == 0 {
		return nil, nil
	}

	url := fmt.Sprintf("%s/api/v1/txs", baseURL)

	// Build request body: concatenated 32-byte txids
	requestBody := make([]byte, len(txIDs)*32)
	for i, txid := range txIDs {
		copy(requestBody[i*32:(i+1)*32], txid[:])
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Parse transactions from response body
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

		txBytes = append(txBytes, tx.Bytes())
	}

	if len(txBytes) != len(txIDs) {
		return nil, fmt.Errorf("transaction count mismatch: requested %d, received %d", len(txIDs), len(txBytes))
	}

	return txBytes, nil
}

// FetchSubtreeTransactions fetches specific transactions from a subtree (Format 2b - subtree-optimized)
// Endpoint: POST {baseURL}/api/v1/subtree/{subtreeHash}/txs
// Request body: concatenated 32-byte transaction IDs to fetch
// Returns the full transaction data for the requested txids
// Use this when fetching many transactions from a single subtree (more efficient server-side)
func FetchSubtreeTransactions(ctx context.Context, baseURL string, subtreeHash kvstore.Hash, txIDs []kvstore.Hash) ([][]byte, error) {
	if len(txIDs) == 0 {
		return nil, nil
	}

	url := fmt.Sprintf("%s/api/v1/subtree/%s/txs", baseURL, chainhash.Hash(subtreeHash).String())

	// Build request body: concatenated 32-byte txids
	requestBody := make([]byte, len(txIDs)*32)
	for i, txid := range txIDs {
		copy(requestBody[i*32:(i+1)*32], txid[:])
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Parse transactions from response body
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

		txBytes = append(txBytes, tx.Bytes())
	}

	if len(txBytes) != len(txIDs) {
		return nil, fmt.Errorf("transaction count mismatch: requested %d, received %d", len(txIDs), len(txBytes))
	}

	return txBytes, nil
}

// FetchMissingSubtreeTransactions intelligently fetches transactions based on cache hit rate
// If most transactions are missing, uses subtree-optimized endpoint
// If only a few are missing, uses specific txid endpoint
// Threshold: if >70% of transactions are missing, use subtree endpoint
func FetchMissingSubtreeTransactions(ctx context.Context, baseURL string, subtreeHash kvstore.Hash, allTxIDs, missingTxIDs []kvstore.Hash) ([][]byte, error) {
	if len(missingTxIDs) == 0 {
		return nil, nil
	}

	missRate := float64(len(missingTxIDs)) / float64(len(allTxIDs))

	// If >70% are missing, fetch all from subtree endpoint (server can optimize)
	// Otherwise fetch specific txids (likely scattered across storage)
	if missRate > 0.7 {
		return FetchSubtreeTransactions(ctx, baseURL, subtreeHash, missingTxIDs)
	}

	return FetchTransactionsByTxID(ctx, baseURL, missingTxIDs)
}
