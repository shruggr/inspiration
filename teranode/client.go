package teranode

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{}}
}

func (c *Client) FetchTransaction(ctx context.Context, txidHex string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/tx/%s", c.baseURL, txidHex)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch tx %s: status %d", txidHex, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// FetchSubtreeData retrieves raw subtree bytes from a URL.
// The caller is responsible for parsing the bytes with go-subtree.
func (c *Client) FetchSubtreeData(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch subtree: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// TxIDToHex converts a 32-byte txid to hex string (reversed, Bitcoin byte-order convention).
func TxIDToHex(txid []byte) string {
	reversed := make([]byte, 32)
	for i, b := range txid {
		reversed[31-i] = b
	}
	return hex.EncodeToString(reversed)
}
