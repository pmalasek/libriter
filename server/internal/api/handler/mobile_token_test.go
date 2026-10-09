package handler

import (
	"context"
	"net/http"
	"testing"

	"libriter/internal/model"
	"libriter/internal/storage"
)

// mobileToken si vyžádá dlouhodobý token stejnou cestou jako mobilní aplikace.
func (e *adminTestEnv) mobileToken(t *testing.T, token string) string {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/auth/mobile-token", token,
		map[string]string{"device_name": "Pixel 8"})
	if rec.Code != http.StatusOK {
		t.Fatalf("mobile-token: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	decodeJSON(t, rec.Body.Bytes(), &body)
	if body.Token == "" || body.ExpiresAt == "" {
		t.Fatalf("prázdný token: %s", rec.Body.String())
	}
	return body.Token
}

// Mobilní token otevírá API, ale ne audio soubory – ty mají vlastní
// krátkodobý token, který jediný smí do adresy.
func TestMobileTokenOpensAPIButNotAudio(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "mobil@example.com", model.RoleReader)
	_, chapterIDs := seedBook(t, env, "Kybiadi", nil, nil, 1)
	writeChapterFile(t, env, "knihovna/Kybiadi/01.mp3", "0123456789")

	mobile := env.mobileToken(t, token)

	if rec := env.do(t, http.MethodGet, "/sessions", mobile, nil); rec.Code != http.StatusOK {
		t.Errorf("API mobilním tokenem: %d %s", rec.Code, rec.Body.String())
	}

	audio := "/chapters/" + chapterIDs[0].String() + "/audio?t=" + mobile
	if rec := env.do(t, http.MethodGet, audio, "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("audio mobilním tokenem: %d, čekáno 401", rec.Code)
	}
}

// Opačný směr: streamovací token do API nesmí.
func TestStreamTokenRejectedByAPI(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "stream-api@example.com", model.RoleReader)

	stream := env.streamToken(t, token)
	if rec := env.do(t, http.MethodGet, "/sessions", stream, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("API streamovacím tokenem: %d, čekáno 401", rec.Code)
	}
}

// Uniklý mobilní token se nesmí sám prodlužovat – další token vyrazí jen
// skutečné přihlášení.
func TestMobileTokenCannotMintAnother(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "retez@example.com", model.RoleReader)

	mobile := env.mobileToken(t, token)
	rec := env.do(t, http.MethodPost, "/auth/mobile-token", mobile, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("řetězení tokenů: %d %s, čekáno 403", rec.Code, rec.Body.String())
	}
}

// Mobilní token smazaného účtu neprojde – middleware se ptá databáze,
// ne tokenu.
func TestMobileTokenDiesWithAccount(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin-mobil@example.com", model.RoleAdmin)
	userToken, userID := env.login(t, "smazany@example.com", model.RoleReader)

	mobile := env.mobileToken(t, userToken)
	if rec := env.do(t, http.MethodDelete, "/users/"+userID, adminToken, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("smazání účtu: %d %s", rec.Code, rec.Body.String())
	}

	if rec := env.do(t, http.MethodGet, "/sessions", mobile, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("token smazaného účtu: %d, čekáno 401", rec.Code)
	}
}

// Seznam kapitol nese velikost souboru – bez ní mobil neví, kolik místa si
// má na knihu připravit.
func TestListChaptersExposesSizeBytes(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "velikost@example.com", model.RoleReader)

	book, err := env.store.CreateBook(context.Background(), storage.BookInput{
		Title: "Marsan", DurationSeconds: 120, FilePath: "knihovna/Marsan", Language: "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	if _, err := env.store.UpsertChapter(context.Background(), storage.ChapterInput{
		BookID: book.ID, Position: 1, Title: "Sol 1",
		FilePath: "knihovna/Marsan/01.mp3", DurationSeconds: 120, SizeBytes: 4_096_000,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	rec := env.do(t, http.MethodGet, "/books/"+book.ID.String()+"/chapters", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("kapitoly: %d %s", rec.Code, rec.Body.String())
	}
	var chapters []chapterResponse
	decodeJSON(t, rec.Body.Bytes(), &chapters)
	if len(chapters) != 1 || chapters[0].SizeBytes != 4_096_000 {
		t.Errorf("size_bytes = %+v, chtěno 4096000", chapters)
	}
}
