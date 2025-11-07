package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shruggr/inspiration/kvstore"
	"github.com/shruggr/inspiration/metadata"
)

// Store is a SQLite-backed implementation of metadata.Store
type Store struct {
	db *sql.DB
}

// Config holds configuration for SQLite
type Config struct {
	DBPath string // Path to SQLite database file
}

// New creates a new SQLite-backed metadata store
func New(config *Config) (*Store, error) {
	if config.DBPath == "" {
		return nil, fmt.Errorf("DBPath is required")
	}

	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	store := &Store{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the necessary tables
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS blocks (
		height INTEGER PRIMARY KEY,
		block_hash BLOB NOT NULL,
		merkle_root BLOB NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_blocks_hash ON blocks(block_hash);
	`

	_, err := s.db.Exec(schema)
	return err
}

// PutBlock stores block metadata
func (s *Store) PutBlock(ctx context.Context, meta *metadata.BlockMeta) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO blocks (height, block_hash, merkle_root) VALUES (?, ?, ?)`,
		meta.Height, meta.BlockHash[:], meta.MerkleRoot[:],
	)
	if err != nil {
		return fmt.Errorf("failed to insert block: %w", err)
	}
	return nil
}

// GetBlock retrieves block metadata by height
func (s *Store) GetBlock(ctx context.Context, height uint64) (*metadata.BlockMeta, error) {
	var meta metadata.BlockMeta
	var blockHash, merkleRoot []byte

	err := s.db.QueryRowContext(ctx,
		`SELECT height, block_hash, merkle_root FROM blocks WHERE height = ?`,
		height,
	).Scan(&meta.Height, &blockHash, &merkleRoot)

	if err == sql.ErrNoRows {
		return nil, nil // Block not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query block: %w", err)
	}

	copy(meta.BlockHash[:], blockHash)
	copy(meta.MerkleRoot[:], merkleRoot)

	return &meta, nil
}

// GetBlockByHash retrieves block metadata by block hash
func (s *Store) GetBlockByHash(ctx context.Context, blockHash kvstore.Hash) (*metadata.BlockMeta, error) {
	var height uint64

	// First get the height
	err := s.db.QueryRowContext(ctx,
		`SELECT height FROM blocks WHERE block_hash = ?`,
		blockHash[:],
	).Scan(&height)

	if err == sql.ErrNoRows {
		return nil, nil // Block not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query block by hash: %w", err)
	}

	// Use GetBlock to get full metadata
	return s.GetBlock(ctx, height)
}

// DeleteBlock removes block metadata (for reorg cleanup)
func (s *Store) DeleteBlock(ctx context.Context, height uint64) error {
	// CASCADE will delete subtrees automatically
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM blocks WHERE height = ?`,
		height,
	)
	return err
}

// GetLatestBlock returns the highest block height stored
func (s *Store) GetLatestBlock(ctx context.Context) (*metadata.BlockMeta, error) {
	var height uint64

	err := s.db.QueryRowContext(ctx,
		`SELECT height FROM blocks ORDER BY height DESC LIMIT 1`,
	).Scan(&height)

	if err == sql.ErrNoRows {
		return nil, nil // No blocks stored
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest block: %w", err)
	}

	return s.GetBlock(ctx, height)
}

// Close releases all database resources
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

