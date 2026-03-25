package txindexer

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

type P2PKHIndexer struct{}

func NewP2PKHIndexer() *P2PKHIndexer { return &P2PKHIndexer{} }

func (p *P2PKHIndexer) Name() string { return "P2PKH" }

func (p *P2PKHIndexer) Index(_ context.Context, txCtx *TransactionContext) ([]*IndexResult, error) {
	tx, err := transaction.NewTransactionFromBytes(txCtx.RawTx)
	if err != nil {
		return nil, err
	}

	addrVouts := make(map[string][]uint32)
	for i, output := range tx.Outputs {
		if output.LockingScript.IsP2PKH() {
			addrs, err := output.LockingScript.Addresses()
			if err != nil || len(addrs) == 0 {
				continue
			}
			addrVouts[addrs[0]] = append(addrVouts[addrs[0]], uint32(i))
		}
	}

	results := make([]*IndexResult, 0, len(addrVouts))
	for addr, vouts := range addrVouts {
		results = append(results, &IndexResult{
			Key:   "address",
			Value: addr,
			Vouts: vouts,
		})
	}
	return results, nil
}
