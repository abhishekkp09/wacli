package main

import (
	"os"
	"time"

	"github.com/abhishekkp09/wacli/internal/out"
	"github.com/abhishekkp09/wacli/internal/store"
	"github.com/spf13/cobra"
)

// The audit reports two independently-paginated sources: revokes, and "syncd"
// (the three app-state kinds combined), matching the wai daemon's shape.
var deletionsSyncdKinds = []string{"delete_for_me", "clear_chat", "delete_chat"}

type deletionsAuditOut struct {
	Since             string                `json:"since"`
	SinceMS           int64                 `json:"since_ms"`
	LatestDeletedAtMS *int64                `json:"latest_deleted_at_ms"`
	Page              deletionsPageOut      `json:"page"`
	Counts            deletionsCountsOut    `json:"counts"`
	Revokes           []revokeEntryOut      `json:"revokes"`
	DeleteForMe       []deleteForMeEntryOut `json:"delete_for_me"`
	ClearChat         []syncdEntryOut       `json:"clear_chat"`
	DeleteChat        []syncdEntryOut       `json:"delete_chat"`
}

type deletionsPageOut struct {
	Limit          int  `json:"limit"`
	Offset         int  `json:"offset"`
	HasMoreRevokes bool `json:"has_more_revokes"`
	HasMoreSyncd   bool `json:"has_more_syncd"`
}

type deletionsCountsOut struct {
	Revoke      int `json:"revoke"`
	DeleteForMe int `json:"delete_for_me"`
	ClearChat   int `json:"clear_chat"`
	DeleteChat  int `json:"delete_chat"`
}

type revokeEntryOut struct {
	ChatID      string `json:"chat_id"`
	ChatJID     string `json:"chat_jid"`
	StanzaID    string `json:"stanza_id"`
	DeletedAt   string `json:"deleted_at"`
	DeletedAtMS int64  `json:"deleted_at_ms"`
}

type deleteForMeEntryOut struct {
	ChatID      string `json:"chat_id"`
	ChatJID     string `json:"chat_jid"`
	StanzaID    string `json:"stanza_id"`
	FromMe      bool   `json:"from_me"`
	DeletedAt   string `json:"deleted_at"`
	DeletedAtMS int64  `json:"deleted_at_ms"`
}

type syncdEntryOut struct {
	ChatID      string `json:"chat_id"`
	ChatJID     string `json:"chat_jid"`
	DeletedAt   string `json:"deleted_at"`
	DeletedAtMS int64  `json:"deleted_at_ms"`
}

func newDeletionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deletions",
		Short: "Inspect recorded message and chat deletions",
	}
	cmd.AddCommand(newDeletionsAuditCmd(flags))
	return cmd
}

func newDeletionsAuditCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List revoke and app-state deletions grouped by kind, with pagination",
		RunE: func(cmd *cobra.Command, args []string) error {
			var since time.Time
			var sinceMS int64
			if sinceStr != "" {
				t, err := parseTime(sinceStr)
				if err != nil {
					return err
				}
				since = t
				sinceMS = t.UTC().UnixMilli()
			}
			if limit <= 0 {
				limit = 100
			}
			if offset < 0 {
				offset = 0
			}

			ctx, cancel := withTimeout(cmd.Context(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			db := a.DB()

			// Fetch limit+1 per source to detect has_more without a COUNT query.
			revRows, err := db.ListDeletions(store.ListDeletionsParams{
				Kinds: []string{"revoke"}, Since: since, Limit: limit + 1, Offset: offset,
			})
			if err != nil {
				return err
			}
			hasMoreRevokes := len(revRows) > limit
			if hasMoreRevokes {
				revRows = revRows[:limit]
			}

			syncRows, err := db.ListDeletions(store.ListDeletionsParams{
				Kinds: deletionsSyncdKinds, Since: since, Limit: limit + 1, Offset: offset,
			})
			if err != nil {
				return err
			}
			hasMoreSyncd := len(syncRows) > limit
			if hasMoreSyncd {
				syncRows = syncRows[:limit]
			}

			var maxMS int64
			haveMax := false
			note := func(t time.Time) {
				ms := t.UTC().UnixMilli()
				if !haveMax || ms > maxMS {
					maxMS, haveMax = ms, true
				}
			}
			fmtTime := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }

			revokes := make([]revokeEntryOut, 0, len(revRows))
			for _, r := range revRows {
				note(r.DeletedAt)
				revokes = append(revokes, revokeEntryOut{
					ChatID:      r.ChatJID,
					ChatJID:     r.ChatJID,
					StanzaID:    r.StanzaID,
					DeletedAt:   fmtTime(r.DeletedAt),
					DeletedAtMS: r.DeletedAt.UTC().UnixMilli(),
				})
			}

			dfm := make([]deleteForMeEntryOut, 0)
			clr := make([]syncdEntryOut, 0)
			del := make([]syncdEntryOut, 0)
			for _, r := range syncRows {
				note(r.DeletedAt)
				switch r.Kind {
				case "delete_for_me":
					dfm = append(dfm, deleteForMeEntryOut{
						ChatID:      r.ChatJID,
						ChatJID:     r.ChatJID,
						StanzaID:    r.StanzaID,
						FromMe:      r.FromMe,
						DeletedAt:   fmtTime(r.DeletedAt),
						DeletedAtMS: r.DeletedAt.UTC().UnixMilli(),
					})
				case "clear_chat":
					clr = append(clr, syncdEntryOut{
						ChatID:      r.ChatJID,
						ChatJID:     r.ChatJID,
						DeletedAt:   fmtTime(r.DeletedAt),
						DeletedAtMS: r.DeletedAt.UTC().UnixMilli(),
					})
				case "delete_chat":
					del = append(del, syncdEntryOut{
						ChatID:      r.ChatJID,
						ChatJID:     r.ChatJID,
						DeletedAt:   fmtTime(r.DeletedAt),
						DeletedAtMS: r.DeletedAt.UTC().UnixMilli(),
					})
				}
			}

			// The resume cursor is populated only once BOTH sources are drained; with
			// ascending order the last page holds the global-max deleted_at.
			var latest *int64
			if !hasMoreRevokes && !hasMoreSyncd && haveMax {
				v := maxMS
				latest = &v
			}

			result := deletionsAuditOut{
				Since:             time.UnixMilli(sinceMS).UTC().Format(time.RFC3339),
				SinceMS:           sinceMS,
				LatestDeletedAtMS: latest,
				Page: deletionsPageOut{
					Limit:          limit,
					Offset:         offset,
					HasMoreRevokes: hasMoreRevokes,
					HasMoreSyncd:   hasMoreSyncd,
				},
				Counts: deletionsCountsOut{
					Revoke:      len(revokes),
					DeleteForMe: len(dfm),
					ClearChat:   len(clr),
					DeleteChat:  len(del),
				},
				Revokes:     revokes,
				DeleteForMe: dfm,
				ClearChat:   clr,
				DeleteChat:  del,
			}
			return out.WriteJSON(os.Stdout, result)
		},
	}

	cmd.Flags().StringVar(&sinceStr, "since", "", "only report deletions at or after this time (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 100, "max entries per source")
	cmd.Flags().IntVar(&offset, "offset", 0, "entries to skip per source")
	return cmd
}
