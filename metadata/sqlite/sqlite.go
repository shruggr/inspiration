package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db *sql.DB
}

func New(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS blocks (
		height          INTEGER NOT NULL,
		block_hash      BLOB NOT NULL PRIMARY KEY,
		header          BLOB NOT NULL,
		tx_count        INTEGER NOT NULL,
		subtree_count   INTEGER NOT NULL,
		index_root      BLOB,
		status          TEXT NOT NULL DEFAULT 'pending',
		created_at      INTEGER DEFAULT (strftime('%s', 'now')),
		promoted_at     INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_blocks_height ON blocks(height);
	CREATE INDEX IF NOT EXISTS idx_blocks_status ON blocks(status);

	CREATE TABLE IF NOT EXISTS block_subtrees (
		block_hash      BLOB NOT NULL,
		subtree_index   INTEGER NOT NULL,
		subtree_hash    BLOB NOT NULL,
		PRIMARY KEY (block_hash, subtree_index),
		FOREIGN KEY (block_hash) REFERENCES blocks(block_hash) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS subtrees (
		subtree_hash    BLOB PRIMARY KEY,
		index_root      BLOB NOT NULL,
		tx_count        INTEGER NOT NULL,
		received_at     INTEGER DEFAULT (strftime('%s', 'now')),
		block_hash      BLOB,
		promoted        INTEGER DEFAULT 0
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) InsertSubtree(ctx context.Context, hash, indexRoot []byte, txCount uint32) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO subtrees (subtree_hash, index_root, tx_count) VALUES (?, ?, ?)`,
		hash, indexRoot, txCount,
	)
	return err
}

func (s *SQLiteStore) InsertBlock(ctx context.Context, height uint32, blockHash, header []byte, txCount uint64, subtreeHashes [][]byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO blocks (height, block_hash, header, tx_count, subtree_count) VALUES (?, ?, ?, ?, ?)`,
		height, blockHash, header, txCount, len(subtreeHashes),
	)
	if err != nil {
		return fmt.Errorf("failed to insert block: %w", err)
	}

	for i, sh := range subtreeHashes {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO block_subtrees (block_hash, subtree_index, subtree_hash) VALUES (?, ?, ?)`,
			blockHash, i, sh,
		)
		if err != nil {
			return fmt.Errorf("failed to insert block_subtree %d: %w", i, err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetBlockSubtrees(ctx context.Context, blockHash []byte) ([][]byte, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT subtree_hash FROM block_subtrees WHERE block_hash = ? ORDER BY subtree_index`,
		blockHash,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hashes [][]byte
	for rows.Next() {
		var h []byte
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}

func (s *SQLiteStore) GetSubtreeIndexRoot(ctx context.Context, subtreeHash []byte) ([]byte, error) {
	var indexRoot []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT index_root FROM subtrees WHERE subtree_hash = ?`,
		subtreeHash,
	).Scan(&indexRoot)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return indexRoot, err
}

func (s *SQLiteStore) SubtreeExists(ctx context.Context, subtreeHash []byte) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM subtrees WHERE subtree_hash = ? LIMIT 1`,
		subtreeHash,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLiteStore) PromoteBlock(ctx context.Context, blockHash []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE blocks SET status = 'confirmed', promoted_at = strftime('%s', 'now') WHERE block_hash = ?`,
		blockHash,
	)
	if err != nil {
		return fmt.Errorf("failed to promote block: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE subtrees SET promoted = 1, block_hash = ? WHERE subtree_hash IN (SELECT subtree_hash FROM block_subtrees WHERE block_hash = ?)`,
		blockHash, blockHash,
	)
	if err != nil {
		return fmt.Errorf("failed to promote subtrees: %w", err)
	}

	return tx.Commit()
}

func (s *SQLiteStore) OrphanBlock(ctx context.Context, blockHash []byte) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE blocks SET status = 'orphaned' WHERE block_hash = ?`,
		blockHash,
	)
	return err
}

func (s *SQLiteStore) GetUnpromotedBlocks(ctx context.Context, deeperThanHeight uint32) ([][]byte, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT block_hash FROM blocks WHERE status = 'pending' AND height <= ? ORDER BY height`,
		deeperThanHeight,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hashes [][]byte
	for rows.Next() {
		var h []byte
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}

func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
