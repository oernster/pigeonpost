package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// FolderUIState returns the account's persisted folder display state: the local custom-folder order
// and the collapsed folder paths. An account with no saved state returns empty slices, which the
// front end reads as "no local preferences yet".
func (s *Store) FolderUIState(ctx context.Context, accountID string) ([]string, []string, error) {
	var orderJSON, collapsedJSON string
	err := s.db.QueryRowContext(ctx,
		"SELECT order_json, collapsed_json FROM folder_ui_state WHERE account_id = ?;", accountID,
	).Scan(&orderJSON, &collapsedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return []string{}, []string{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load folder ui state %q: %w", accountID, err)
	}
	order, err := decodePaths(orderJSON)
	if err != nil {
		return nil, nil, fmt.Errorf("decode folder order %q: %w", accountID, err)
	}
	collapsed, err := decodePaths(collapsedJSON)
	if err != nil {
		return nil, nil, fmt.Errorf("decode collapsed folders %q: %w", accountID, err)
	}
	return order, collapsed, nil
}

// SaveFolderUIState upserts the account's folder display state, replacing whatever was stored. The
// full state is written each time (not a delta), so the stored row is always internally consistent.
func (s *Store) SaveFolderUIState(ctx context.Context, accountID string, order, collapsed []string) error {
	orderJSON, err := encodePaths(order)
	if err != nil {
		return fmt.Errorf("encode folder order %q: %w", accountID, err)
	}
	collapsedJSON, err := encodePaths(collapsed)
	if err != nil {
		return fmt.Errorf("encode collapsed folders %q: %w", accountID, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO folder_ui_state (account_id, order_json, collapsed_json) VALUES (?, ?, ?)
		 ON CONFLICT(account_id) DO UPDATE SET order_json = excluded.order_json,
		 collapsed_json = excluded.collapsed_json;`,
		accountID, orderJSON, collapsedJSON)
	if err != nil {
		return fmt.Errorf("save folder ui state %q: %w", accountID, err)
	}
	return nil
}

// encodePaths marshals a path list to its stored JSON form, writing an empty list (never null) for a
// nil slice so the stored text always decodes back to a slice.
func encodePaths(paths []string) (string, error) {
	if paths == nil {
		paths = []string{}
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// decodePaths unmarshals a stored JSON path list, mapping a JSON null to an empty slice.
func decodePaths(text string) ([]string, error) {
	var paths []string
	if err := json.Unmarshal([]byte(text), &paths); err != nil {
		return nil, err
	}
	if paths == nil {
		paths = []string{}
	}
	return paths, nil
}
