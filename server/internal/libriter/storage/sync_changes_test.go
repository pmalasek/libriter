package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"mediater/internal/libriter/model"
)

func TestSyncChanges(t *testing.T) {
	f := newListFixture(t)
	ctx := context.Background()
	user := mergeTestUser(t, f.store, "sync@example.com")

	// Bez since: celá knihovna, po dávkách.
	full, err := f.store.SyncChanges(ctx, user.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !full.FullResync || full.More || len(full.Books) != 5 || len(full.Authors) != 2 || len(full.Series) != 1 {
		t.Fatalf("plná synchronizace: resync=%v more=%v knihy=%d autoři=%d série=%d",
			full.FullResync, full.More, len(full.Books), len(full.Authors), len(full.Series))
	}
	if len(full.Books[0].Authors) == 0 && full.Books[0].Title != "Anonym" {
		t.Errorf("kniha bez autorů: %+v", full.Books[0])
	}

	// Po dávkách se projde totéž bez ztrát a duplicit.
	seen := map[uuid.UUID]bool{}
	var rev int64
	for i := 0; ; i++ {
		part, err := f.store.SyncChanges(ctx, user.ID, rev, 3)
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && part.FullResync {
			t.Fatalf("dávka %d chce full resync", i)
		}
		for _, b := range part.Books {
			if seen[b.ID] {
				t.Errorf("kniha %s dvakrát", b.Title)
			}
			seen[b.ID] = true
		}
		rev = part.Rev
		if !part.More {
			break
		}
	}
	if len(seen) != 5 {
		t.Errorf("po dávkách %d knih", len(seen))
	}

	// Žádná změna → prázdná dávka se stejným rev.
	empty, _ := f.store.SyncChanges(ctx, user.ID, rev, 100)
	if empty.FullResync || len(empty.Books)+len(empty.Authors)+len(empty.Deleted) != 0 || empty.Rev != rev {
		t.Fatalf("bez změn: %+v", empty)
	}

	// Přejmenování autora pošle autora i jeho knihy; smazání tombstone.
	if _, err := f.store.UpdateAuthor(ctx, f.capek.ID, AuthorInput{Name: model.AuthorName{First: "Karel", Last: "Čapek ml."}}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.DeleteBook(ctx, f.books["Zima"]); err != nil {
		t.Fatal(err)
	}
	insertProgress(t, f.store, user.ID, f.books["Jaro"], true)

	delta, err := f.store.SyncChanges(ctx, user.ID, rev, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(delta.Authors) != 1 || len(delta.Books) != 2 || len(delta.Progress) != 1 {
		t.Errorf("změny: autoři=%d knihy=%d progress=%d", len(delta.Authors), len(delta.Books), len(delta.Progress))
	}
	if len(delta.Deleted) != 1 || delta.Deleted[0].Kind != "book" || delta.Deleted[0].ID != f.books["Zima"] {
		t.Errorf("smazané: %+v", delta.Deleted)
	}

	// Cizí stav poslechu se neposílá.
	other := mergeTestUser(t, f.store, "other@example.com")
	if d, _ := f.store.SyncChanges(ctx, other.ID, rev, 100); len(d.Progress) != 0 {
		t.Errorf("cizí progress: %+v", d.Progress)
	}

	// Po úklidu tombstonů musí starý klient stáhnout vše.
	if _, err := f.store.PruneDeletedItems(ctx, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if d, _ := f.store.SyncChanges(ctx, user.ID, rev, 100); !d.FullResync {
		t.Errorf("po úklidu čekám full_resync")
	}
	if d, _ := f.store.SyncChanges(ctx, user.ID, delta.Rev, 100); d.FullResync {
		t.Errorf("aktuální klient full_resync nepotřebuje")
	}
}
