package app

import (
	"context"
	"testing"
	"time"

	"github.com/abhishekkp09/wacli/internal/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestDeleteForMeEventRecordsDeletion(t *testing.T) {
	a := newTestApp(t)
	a.wa = newFakeWA()
	chat := types.NewJID("15551234567", types.DefaultUserServer)
	base := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	a.handleDeleteForMeEvent(context.Background(), &events.DeleteForMe{
		ChatJID:   chat,
		MessageID: "3EB0DFM",
		Timestamp: base,
		IsFromMe:  true,
	})

	rows, err := a.db.ListDeletions(store.ListDeletionsParams{Kinds: []string{"delete_for_me"}})
	if err != nil {
		t.Fatalf("ListDeletions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("delete_for_me rows = %d, want 1", len(rows))
	}
	d := rows[0]
	if d.ChatJID != chat.String() || d.StanzaID != "3EB0DFM" || !d.FromMe || !d.DeletedAt.Equal(base) {
		t.Fatalf("delete_for_me deletion = %+v (chat %s)", d, chat.String())
	}
}

func TestClearChatEventRecordsDeletion(t *testing.T) {
	a := newTestApp(t)
	a.wa = newFakeWA()
	chat := types.NewJID("15551234567", types.DefaultUserServer)
	base := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	a.handleClearChatEvent(context.Background(), &events.ClearChat{JID: chat, Timestamp: base})

	rows, err := a.db.ListDeletions(store.ListDeletionsParams{Kinds: []string{"clear_chat"}})
	if err != nil {
		t.Fatalf("ListDeletions: %v", err)
	}
	if len(rows) != 1 || rows[0].ChatJID != chat.String() || rows[0].StanzaID != "" {
		t.Fatalf("clear_chat deletion = %+v", rows)
	}
}

func TestDeleteChatEventRecordsDeletion(t *testing.T) {
	a := newTestApp(t)
	a.wa = newFakeWA()
	chat := types.NewJID("15551234567", types.DefaultUserServer)
	base := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	a.handleDeleteChatEvent(context.Background(), &events.DeleteChat{JID: chat, Timestamp: base})

	rows, err := a.db.ListDeletions(store.ListDeletionsParams{Kinds: []string{"delete_chat"}})
	if err != nil {
		t.Fatalf("ListDeletions: %v", err)
	}
	if len(rows) != 1 || rows[0].ChatJID != chat.String() || rows[0].StanzaID != "" {
		t.Fatalf("delete_chat deletion = %+v", rows)
	}
}
