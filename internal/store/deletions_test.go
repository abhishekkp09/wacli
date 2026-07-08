package store

import (
	"fmt"
	"testing"
	"time"
)

func TestDeletionsInsertAndListByKind(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("InsertDeletion: %v", err)
		}
	}
	must(db.InsertDeletion(InsertDeletionParams{Kind: "revoke", ChatJID: "a@s.whatsapp.net", StanzaID: "3EB0AAA", DeletedAt: base}))
	must(db.InsertDeletion(InsertDeletionParams{Kind: "delete_for_me", ChatJID: "b@s.whatsapp.net", StanzaID: "3EB0BBB", FromMe: true, DeletedAt: base.Add(time.Minute)}))
	must(db.InsertDeletion(InsertDeletionParams{Kind: "clear_chat", ChatJID: "c@s.whatsapp.net", DeletedAt: base.Add(2 * time.Minute)}))
	must(db.InsertDeletion(InsertDeletionParams{Kind: "delete_chat", ChatJID: "d@s.whatsapp.net", DeletedAt: base.Add(3 * time.Minute)}))

	revokes, err := db.ListDeletions(ListDeletionsParams{Kinds: []string{"revoke"}})
	if err != nil {
		t.Fatalf("ListDeletions revoke: %v", err)
	}
	if len(revokes) != 1 || revokes[0].StanzaID != "3EB0AAA" || revokes[0].ChatJID != "a@s.whatsapp.net" {
		t.Fatalf("revokes = %+v", revokes)
	}

	syncd, err := db.ListDeletions(ListDeletionsParams{Kinds: []string{"delete_for_me", "clear_chat", "delete_chat"}})
	if err != nil {
		t.Fatalf("ListDeletions syncd: %v", err)
	}
	if len(syncd) != 3 {
		t.Fatalf("syncd len = %d, want 3: %+v", len(syncd), syncd)
	}
	for _, d := range syncd {
		switch d.Kind {
		case "delete_for_me":
			if !d.FromMe || d.StanzaID != "3EB0BBB" {
				t.Fatalf("delete_for_me = %+v", d)
			}
		case "clear_chat", "delete_chat":
			if d.StanzaID != "" || d.FromMe {
				t.Fatalf("chat-level kind carries stanza/from_me: %+v", d)
			}
		}
	}
	// Ascending order by deleted_at.
	if !(syncd[0].DeletedAt.Before(syncd[1].DeletedAt) && syncd[1].DeletedAt.Before(syncd[2].DeletedAt)) {
		t.Fatalf("syncd not ascending: %+v", syncd)
	}
}

func TestDeletionsInsertIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	p := InsertDeletionParams{Kind: "revoke", ChatJID: "a@s.whatsapp.net", StanzaID: "3EB0AAA", DeletedAt: base}
	for i := 0; i < 3; i++ {
		if err := db.InsertDeletion(p); err != nil {
			t.Fatalf("InsertDeletion #%d: %v", i, err)
		}
	}
	rows, err := db.ListDeletions(ListDeletionsParams{Kinds: []string{"revoke"}})
	if err != nil {
		t.Fatalf("ListDeletions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("dedup failed: %d rows", len(rows))
	}
}

func TestDeletionsSinceAndPagination(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if err := db.InsertDeletion(InsertDeletionParams{
			Kind:      "revoke",
			ChatJID:   "a@s.whatsapp.net",
			StanzaID:  fmt.Sprintf("id-%d", i),
			DeletedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("InsertDeletion: %v", err)
		}
	}

	// Since is inclusive (>=): rows at +3m and +4m.
	got, err := db.ListDeletions(ListDeletionsParams{Kinds: []string{"revoke"}, Since: base.Add(3 * time.Minute)})
	if err != nil {
		t.Fatalf("ListDeletions since: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("since filter = %d rows, want 2", len(got))
	}

	page1, err := db.ListDeletions(ListDeletionsParams{Kinds: []string{"revoke"}, Limit: 2})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 || page1[0].StanzaID != "id-0" {
		t.Fatalf("page1 = %+v", page1)
	}
	page2, err := db.ListDeletions(ListDeletionsParams{Kinds: []string{"revoke"}, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 2 || page2[0].StanzaID != "id-2" {
		t.Fatalf("page2 = %+v", page2)
	}
}
