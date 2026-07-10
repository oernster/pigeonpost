package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ListContactGroups returns every group with its member ids, ordered by name.
func (s *Store) ListContactGroups(ctx context.Context) ([]domain.ContactGroup, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name FROM contact_group ORDER BY name;")
	if err != nil {
		return nil, fmt.Errorf("query contact groups: %w", err)
	}
	type groupRow struct{ id, name string }
	var bases []groupRow
	for rows.Next() {
		var b groupRow
		if err := rows.Scan(&b.id, &b.name); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan contact group: %w", err)
		}
		bases = append(bases, b)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate contact groups: %w", err)
	}
	// Close before loading members, since each group issues a further query on the same connection.
	_ = rows.Close()

	groups := make([]domain.ContactGroup, 0, len(bases))
	for _, b := range bases {
		members, err := s.groupMembers(ctx, b.id)
		if err != nil {
			return nil, err
		}
		group, err := domain.NewContactGroup(b.id, b.name, members)
		if err != nil {
			return nil, fmt.Errorf("rebuild contact group %q: %w", b.id, err)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// groupMembers returns a group's member contact ids in stored order.
func (s *Store) groupMembers(ctx context.Context, groupID string) ([]string, error) {
	return queryRows(ctx, s.db, "group members",
		"SELECT contact_id FROM contact_group_member WHERE group_id = ? ORDER BY position;",
		func(row scanner) (string, error) {
			var id string
			if err := row.Scan(&id); err != nil {
				return "", fmt.Errorf("scan group member: %w", err)
			}
			return id, nil
		}, groupID)
}

// SaveContactGroup inserts or updates a group and replaces its member links in one transaction.
func (s *Store) SaveContactGroup(ctx context.Context, g domain.ContactGroup) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO contact_group (id, name) VALUES (?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name;`,
			g.ID(), g.Name()); err != nil {
			return fmt.Errorf("save contact group %q: %w", g.ID(), err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM contact_group_member WHERE group_id = ?;", g.ID()); err != nil {
			return fmt.Errorf("clear group members: %w", err)
		}
		for i, member := range g.Members() {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO contact_group_member (group_id, contact_id, position) VALUES (?, ?, ?);",
				g.ID(), member, i); err != nil {
				return fmt.Errorf("insert group member: %w", err)
			}
		}
		return nil
	})
}

// DeleteContactGroup removes a group and its member links in one transaction. The contacts themselves
// are untouched.
func (s *Store) DeleteContactGroup(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM contact_group_member WHERE group_id = ?;", id); err != nil {
			return fmt.Errorf("detach group members: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM contact_group WHERE id = ?;", id); err != nil {
			return fmt.Errorf("delete contact group %q: %w", id, err)
		}
		return nil
	})
}
