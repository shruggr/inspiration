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
		height       INTEGER NOT NULL,
		block_hash   BLOB NOT NULL,
		merkle_root  BLOB PRIMARY KEY,
		tx_count     INTEGER NOT NULL,
		status       TEXT NOT NULL DEFAULT 'main',
		timestamp    INTEGER,
		created_at   INTEGER DEFAULT (strftime('%s', 'now'))
	);

	CREATE INDEX IF NOT EXISTS idx_blocks_status_height ON blocks(status, height);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_blocks_hash ON blocks(block_hash);

	CREATE TABLE IF NOT EXISTS subtrees (
		merkle_root         BLOB NOT NULL,
		subtree_index       INTEGER NOT NULL,
		subtree_merkle_root BLOB NOT NULL,
		tx_count            INTEGER NOT NULL,
		index_root          BLOB NOT NULL,
		tx_tree_root        BLOB NOT NULL,

		PRIMARY KEY (merkle_root, subtree_index),
		FOREIGN KEY (merkle_root) REFERENCES blocks(merkle_root) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_subtrees_merkle_root_subtree_index ON subtrees(merkle_root, subtree_index);
	`

	_, err := s.db.Exec(schema)
	return err
}

// PutBlock stores block metadata with associated subtrees atomically
func (s *Store) PutBlock(ctx context.Context, block *metadata.BlockMeta, subtrees []*metadata.SubtreeMeta) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO blocks (height, block_hash, merkle_root, tx_count, status, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		block.Height, block.BlockHash[:], block.MerkleRoot[:], block.TxCount, string(block.Status), block.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert block: %w", err)
	}

	for _, subtree := range subtrees {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO subtrees (merkle_root, subtree_index, subtree_merkle_root, tx_count, index_root, tx_tree_root)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			subtree.MerkleRoot[:], subtree.SubtreeIndex, subtree.SubtreeMerkleRoot[:],
			subtree.TxCount, subtree.IndexRoot, subtree.TxTreeRoot,
		)
		if err != nil {
			return fmt.Errorf("failed to insert subtree %d: %w", subtree.SubtreeIndex, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetBlock retrieves block metadata by height
func (s *Store) GetBlock(ctx context.Context, height uint64) (*metadata.BlockMeta, error) {
	var meta metadata.BlockMeta
	var blockHash, merkleRoot []byte
	var status string
	var timestamp sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT height, block_hash, merkle_root, tx_count, status, timestamp
		 FROM blocks WHERE height = ? AND status = 'main'`,
		height,
	).Scan(&meta.Height, &blockHash, &merkleRoot, &meta.TxCount, &status, &timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query block: %w", err)
	}

	copy(meta.BlockHash[:], blockHash)
	copy(meta.MerkleRoot[:], merkleRoot)
	meta.Status = metadata.BlockStatus(status)
	if timestamp.Valid {
		meta.Timestamp = timestamp.Int64
	}

	return &meta, nil
}

// GetBlockByHash retrieves block metadata by block hash
func (s *Store) GetBlockByHash(ctx context.Context, blockHash kvstore.Hash) (*metadata.BlockMeta, error) {
	var meta metadata.BlockMeta
	var blockHashBytes, merkleRoot []byte
	var status string
	var timestamp sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT height, block_hash, merkle_root, tx_count, status, timestamp
		 FROM blocks WHERE block_hash = ?`,
		blockHash[:],
	).Scan(&meta.Height, &blockHashBytes, &merkleRoot, &meta.TxCount, &status, &timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query block by hash: %w", err)
	}

	copy(meta.BlockHash[:], blockHashBytes)
	copy(meta.MerkleRoot[:], merkleRoot)
	meta.Status = metadata.BlockStatus(status)
	if timestamp.Valid {
		meta.Timestamp = timestamp.Int64
	}

	return &meta, nil
}

// GetBlockByMerkleRoot retrieves block metadata by merkle root
func (s *Store) GetBlockByMerkleRoot(ctx context.Context, merkleRoot kvstore.Hash) (*metadata.BlockMeta, error) {
	var meta metadata.BlockMeta
	var blockHash, merkleRootBytes []byte
	var status string
	var timestamp sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT height, block_hash, merkle_root, tx_count, status, timestamp
		 FROM blocks WHERE merkle_root = ?`,
		merkleRoot[:],
	).Scan(&meta.Height, &blockHash, &merkleRootBytes, &meta.TxCount, &status, &timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query block by merkle root: %w", err)
	}

	copy(meta.BlockHash[:], blockHash)
	copy(meta.MerkleRoot[:], merkleRootBytes)
	meta.Status = metadata.BlockStatus(status)
	if timestamp.Valid {
		meta.Timestamp = timestamp.Int64
	}

	return &meta, nil
}

// GetSubtrees retrieves all subtrees for a block, ordered by subtree_index
func (s *Store) GetSubtrees(ctx context.Context, merkleRoot kvstore.Hash) ([]*metadata.SubtreeMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT merkle_root, subtree_index, subtree_merkle_root, tx_count, index_root, tx_tree_root
		 FROM subtrees WHERE merkle_root = ? ORDER BY subtree_index`,
		merkleRoot[:],
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query subtrees: %w", err)
	}
	defer rows.Close()

	var subtrees []*metadata.SubtreeMeta
	for rows.Next() {
		var subtree metadata.SubtreeMeta
		var merkleRootBytes, subtreeMerkleRoot []byte

		err := rows.Scan(&merkleRootBytes, &subtree.SubtreeIndex, &subtreeMerkleRoot,
			&subtree.TxCount, &subtree.IndexRoot, &subtree.TxTreeRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subtree: %w", err)
		}

		copy(subtree.MerkleRoot[:], merkleRootBytes)
		copy(subtree.SubtreeMerkleRoot[:], subtreeMerkleRoot)

		subtrees = append(subtrees, &subtree)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subtrees: %w", err)
	}

	return subtrees, nil
}

// MarkOrphan marks blocks at the given height as orphaned
func (s *Store) MarkOrphan(ctx context.Context, height uint64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE blocks SET status = 'orphan' WHERE height = ? AND status = 'main'`,
		height,
	)
	if err != nil {
		return fmt.Errorf("failed to mark blocks as orphan: %w", err)
	}
	return nil
}

// CleanupOrphans removes orphaned blocks older than the given depth
func (s *Store) CleanupOrphans(ctx context.Context, currentHeight uint64, depth uint64) error {
	if currentHeight < depth {
		return nil
	}

	cutoffHeight := currentHeight - depth

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM blocks WHERE status = 'orphan' AND height <= ?`,
		cutoffHeight,
	)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphans: %w", err)
	}

	return nil
}

// GetLatestBlock returns the highest main chain block
func (s *Store) GetLatestBlock(ctx context.Context) (*metadata.BlockMeta, error) {
	var meta metadata.BlockMeta
	var blockHash, merkleRoot []byte
	var status string
	var timestamp sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT height, block_hash, merkle_root, tx_count, status, timestamp
		 FROM blocks WHERE status = 'main' ORDER BY height DESC LIMIT 1`,
	).Scan(&meta.Height, &blockHash, &merkleRoot, &meta.TxCount, &status, &timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest block: %w", err)
	}

	copy(meta.BlockHash[:], blockHash)
	copy(meta.MerkleRoot[:], merkleRoot)
	meta.Status = metadata.BlockStatus(status)
	if timestamp.Valid {
		meta.Timestamp = timestamp.Int64
	}

	return &meta, nil
}

// Close releases all database resources
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

