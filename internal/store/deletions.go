package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Deletion is one recorded deletion event, mirroring the wai daemon's audit rows.
// StanzaID is the revoked/deleted message id for revoke/delete_for_me and empty
// for the chat-level clear_chat/delete_chat. FromMe is meaningful only for
// delete_for_me.
type Deletion struct {
	Kind      string
	ChatJID   string
	StanzaID  string
	FromMe    bool
	DeletedAt time.Time
}

type InsertDeletionParams struct {
	Kind      string
	ChatJID   string
	StanzaID  string
	FromMe    bool
	DeletedAt time.Time
}

type ListDeletionsParams struct {
	Kinds  []string
	Since  time.Time
	Limit  int
	Offset int
}

// InsertDeletion records a deletion. from_me is stored only for delete_for_me
// (NULL otherwise). The UNIQUE(kind, chat_jid, stanza_id, deleted_at) constraint
// makes a deletion re-delivered across syncs idempotent (INSERT OR IGNORE).
func (d *DB) InsertDeletion(p InsertDeletionParams) error {
	kind := strings.TrimSpace(p.Kind)
	if kind == "" {
		return fmt.Errorf("deletion kind is required")
	}
	chatJID := strings.TrimSpace(p.ChatJID)
	if chatJID == "" {
		return fmt.Errorf("deletion chat JID is required")
	}
	var fromMe interface{}
	if kind == "delete_for_me" {
		fromMe = boolToInt(p.FromMe)
	}
	_, err := d.sql.Exec(`
		INSERT OR IGNORE INTO deletions(kind, chat_jid, stanza_id, from_me, deleted_at)
		VALUES(?, ?, ?, ?, ?)
	`, kind, chatJID, strings.TrimSpace(p.StanzaID), fromMe, unix(p.DeletedAt))
	return err
}

// ListDeletions returns deletions filtered by kind and since, ordered ascending
// by (deleted_at, rowid) so the final page holds the global-max deleted_at (which
// the caller relies on for the drained-only latest_deleted_at_ms cursor).
func (d *DB) ListDeletions(p ListDeletionsParams) ([]Deletion, error) {
	query := `SELECT kind, chat_jid, COALESCE(stanza_id,''), from_me, deleted_at FROM deletions WHERE 1=1`
	var args []interface{}
	if len(p.Kinds) > 0 {
		ph := make([]string, len(p.Kinds))
		for i, k := range p.Kinds {
			ph[i] = "?"
			args = append(args, k)
		}
		query += " AND kind IN (" + strings.Join(ph, ",") + ")"
	}
	if !p.Since.IsZero() {
		query += " AND deleted_at >= ?"
		args = append(args, unix(p.Since))
	}
	query += " ORDER BY deleted_at ASC, rowid ASC"
	if p.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, p.Limit)
		if p.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, p.Offset)
		}
	}
	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Deletion
	for rows.Next() {
		var del Deletion
		var ts int64
		var fromMe sql.NullInt64
		if err := rows.Scan(&del.Kind, &del.ChatJID, &del.StanzaID, &fromMe, &ts); err != nil {
			return nil, err
		}
		del.DeletedAt = fromUnix(ts)
		del.FromMe = fromMe.Valid && fromMe.Int64 != 0
		out = append(out, del)
	}
	return out, rows.Err()
}
