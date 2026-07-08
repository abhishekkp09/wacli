package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/abhishekkp09/wacli/internal/store"
)

func TestDeletionsAuditCommand(t *testing.T) {
	storeDir := t.TempDir()
	db, err := store.Open(storeDir + "/wacli.db")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	base := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	seed := []store.InsertDeletionParams{
		{Kind: "revoke", ChatJID: "a@s.whatsapp.net", StanzaID: "3EB0R1", DeletedAt: base},
		{Kind: "revoke", ChatJID: "a@s.whatsapp.net", StanzaID: "3EB0R2", DeletedAt: base.Add(time.Minute)},
		{Kind: "delete_for_me", ChatJID: "b@s.whatsapp.net", StanzaID: "3EB0D1", FromMe: true, DeletedAt: base.Add(2 * time.Minute)},
		{Kind: "clear_chat", ChatJID: "c@s.whatsapp.net", DeletedAt: base.Add(3 * time.Minute)},
		{Kind: "delete_chat", ChatJID: "d@s.whatsapp.net", DeletedAt: base.Add(4 * time.Minute)},
	}
	for _, p := range seed {
		if err := db.InsertDeletion(p); err != nil {
			t.Fatalf("InsertDeletion: %v", err)
		}
	}
	_ = db.Close()

	type entry struct {
		ChatID      string `json:"chat_id"`
		ChatJID     string `json:"chat_jid"`
		StanzaID    string `json:"stanza_id"`
		FromMe      bool   `json:"from_me"`
		DeletedAtMS int64  `json:"deleted_at_ms"`
	}
	type data struct {
		LatestDeletedAtMS *int64 `json:"latest_deleted_at_ms"`
		Page              struct {
			HasMoreRevokes bool `json:"has_more_revokes"`
			HasMoreSyncd   bool `json:"has_more_syncd"`
		} `json:"page"`
		Counts struct {
			Revoke      int `json:"revoke"`
			DeleteForMe int `json:"delete_for_me"`
			ClearChat   int `json:"clear_chat"`
			DeleteChat  int `json:"delete_chat"`
		} `json:"counts"`
		Revokes     []entry `json:"revokes"`
		DeleteForMe []entry `json:"delete_for_me"`
		ClearChat   []entry `json:"clear_chat"`
		DeleteChat  []entry `json:"delete_chat"`
	}
	type envelope struct {
		Success bool `json:"success"`
		Data    data `json:"data"`
	}

	run := func(args ...string) envelope {
		cmd := newDeletionsAuditCmd(&rootFlags{storeDir: storeDir, timeout: time.Minute})
		cmd.SetArgs(args)
		raw := captureRootStdout(t, func() {
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
		var env envelope
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			t.Fatalf("unmarshal %q: %v", raw, err)
		}
		return env
	}

	// Full page: all sources drained -> latest set, groups partitioned, counts match.
	full := run()
	if !full.Success {
		t.Fatalf("success=false")
	}
	if len(full.Data.Revokes) != 2 || len(full.Data.DeleteForMe) != 1 || len(full.Data.ClearChat) != 1 || len(full.Data.DeleteChat) != 1 {
		t.Fatalf("group partition wrong: %+v", full.Data)
	}
	if full.Data.Counts.Revoke != 2 || full.Data.Counts.DeleteForMe != 1 || full.Data.Counts.ClearChat != 1 || full.Data.Counts.DeleteChat != 1 {
		t.Fatalf("counts wrong: %+v", full.Data.Counts)
	}
	if full.Data.Revokes[0].ChatID != full.Data.Revokes[0].ChatJID {
		t.Fatalf("chat_id != chat_jid: %+v", full.Data.Revokes[0])
	}
	if !full.Data.DeleteForMe[0].FromMe || full.Data.DeleteForMe[0].StanzaID != "3EB0D1" {
		t.Fatalf("delete_for_me entry wrong: %+v", full.Data.DeleteForMe[0])
	}
	if full.Data.ClearChat[0].StanzaID != "" {
		t.Fatalf("clear_chat carries stanza id: %+v", full.Data.ClearChat[0])
	}
	if full.Data.Page.HasMoreRevokes || full.Data.Page.HasMoreSyncd {
		t.Fatalf("has_more should be false when drained")
	}
	wantLatest := base.Add(4 * time.Minute).UTC().UnixMilli()
	if full.Data.LatestDeletedAtMS == nil || *full.Data.LatestDeletedAtMS != wantLatest {
		t.Fatalf("latest_deleted_at_ms = %v, want %d", full.Data.LatestDeletedAtMS, wantLatest)
	}

	// Pagination: limit 1 -> both sources have more; latest is null while paging.
	paged := run("--limit", "1")
	if !paged.Data.Page.HasMoreRevokes || !paged.Data.Page.HasMoreSyncd {
		t.Fatalf("expected has_more on both sources with limit 1: %+v", paged.Data.Page)
	}
	if paged.Data.LatestDeletedAtMS != nil {
		t.Fatalf("latest should be null while paging, got %v", *paged.Data.LatestDeletedAtMS)
	}
	if len(paged.Data.Revokes) != 1 {
		t.Fatalf("limit 1 revokes = %d", len(paged.Data.Revokes))
	}
}
