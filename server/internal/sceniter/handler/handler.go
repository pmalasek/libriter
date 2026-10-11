// Package handler obsahuje HTTP API Sceniteru (/api/v1/sceniter/...).
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mediater/internal/core/httpx"
	"mediater/internal/core/imgthumb"
	"mediater/internal/core/middleware"
	coremodel "mediater/internal/core/model"
	coreservice "mediater/internal/core/service"
	"mediater/internal/sceniter/scanner"
	"mediater/internal/sceniter/service"
	"mediater/internal/sceniter/storage"
	"mediater/internal/sceniter/stream"
)

var (
	writeJSON  = httpx.WriteJSON
	writeError = httpx.WriteError
	readJSON   = httpx.ReadJSON
	parseUUID  = httpx.ParseUUID
)

// Audit akce Sceniteru.
const (
	AuditRescan    = "sceniter.scanner_rescan"
	AuditMatch     = "sceniter.match"
	AuditUnmatch   = "sceniter.unmatch"
	AuditEdit      = "sceniter.edit"
	AuditDetach    = "sceniter.detach_file"
	AuditRenumber  = "sceniter.renumber"
	AuditTrash     = "sceniter.trash_file"
	AuditRestore   = "sceniter.trash_restore"
	AuditPurge     = "sceniter.trash_delete"
	AuditEmpty     = "sceniter.trash_empty"
	auditTargetLib = "sceniter"
)

// Deps jsou závislosti handlerů.
type Deps struct {
	Store     *storage.Store
	Library   *service.Library
	Matcher   *service.Matcher
	Images    *service.TitleImages
	Libs      *service.Libraries
	Scanner   *scanner.Scanner // nil, když je Sceniter vypnutý
	HLS       *stream.Manager
	Subs      *stream.Subtitles
	Auth      *coreservice.AuthService
	Audit     *coreservice.AuditService
	VideoRoot string
	ImageRoot string
	MaxHeight int
	// Thumbs vytváří zmenšeniny plakátů a obrázků (?w=); nil = jen originály.
	Thumbs *imgthumb.Cache
}

// Handler obsluhuje API Sceniteru.
type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) enabled() bool { return h.Scanner != nil }

// requireEnabled odpoví 503, když videoknihovna není nastavená (VIDEO_ROOT).
func (h *Handler) requireEnabled(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.enabled() {
			writeError(w, http.StatusServiceUnavailable, "sceniter.disabled", "videoknihovna není nastavená (VIDEO_ROOT)")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Routes zaregistruje API pod /api/v1/sceniter.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/sceniter", func(r chi.Router) {
		// Stav produktu – i když je vypnutý, ať web umí říct proč.
		r.With(middleware.Authenticate(h.Auth)).Get("/config", h.Config)

		r.Group(func(r chi.Router) {
			r.Use(h.requireEnabled)

			// --- bez přihlášení: obrázky (UUID/název je ochrana) a přehrávání
			// s tokenem v adrese (<video> neumí poslat hlavičku) ---
			r.Get("/images/{name}", h.Image)
			r.Get("/media/{id}/stream", h.Stream)
			r.Head("/media/{id}/stream", h.Stream)
			r.Get("/media/{id}/hls/master.m3u8", h.Master)
			r.Post("/media/{id}/hls/keepalive", h.HLSKeepAlive)
			r.Get("/media/{id}/subtitles/{key}", h.Subtitle)
			r.Get("/hls/{session}/{file}", h.HLSFile)
			r.Delete("/hls/{session}", h.HLSStop)
			r.Post("/hls/{session}/stop", h.HLSStop) // navigator.sendBeacon umí jen POST

			r.Group(func(r chi.Router) {
				r.Use(middleware.Authenticate(h.Auth))

				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(coremodel.RoleReader))
					r.Get("/home", h.Home)
					r.Get("/libraries", h.Libraries)
					r.Get("/movies", h.Movies)
					r.Get("/movies/{id}", h.Movie)
					r.Get("/movies/{id}/similar", h.SimilarMovies)
					r.Get("/shows", h.Shows)
					r.Get("/shows/{id}", h.Show)
					r.Get("/shows/{id}/seasons/{number}", h.Season)
					r.Get("/shows/{id}/similar", h.SimilarShows)
					r.Get("/people/{id}", h.Person)
					r.Get("/episodes/{id}", h.Episode)
					r.Get("/genres", h.Genres)
					r.Get("/search", h.Search)
					r.Get("/media/{id}/playback", h.Playback)

					r.Get("/progress", h.ListProgress)
					r.Put("/progress", h.SaveProgress)
					r.Post("/progress/sync", h.SyncProgress)
					r.Put("/progress/{kind}/{id}/watched", h.MarkWatched)
					r.Delete("/progress/{kind}/{id}", h.ResetProgress)

					r.Get("/watchlist", h.Watchlist)
					r.Put("/watchlist/{kind}/{id}", h.SetWatchlist)
					r.Delete("/watchlist/{kind}/{id}", h.SetWatchlist)
					r.Put("/ratings/{kind}/{id}", h.SetRating)
				})

				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(coremodel.RoleEditor))
					r.Patch("/movies/{id}", h.PatchTitle)
					r.Patch("/shows/{id}", h.PatchTitle)
					r.Put("/movies/{id}/images/{image}", h.UploadTitleImage)
					r.Put("/shows/{id}/images/{image}", h.UploadTitleImage)
					r.Delete("/movies/{id}/images/{image}", h.RemoveTitleImage)
					r.Delete("/shows/{id}/images/{image}", h.RemoveTitleImage)
					r.Get("/metadata/providers", h.MetadataProviders)
					r.Get("/metadata/search", h.MetadataSearch)
					r.Put("/movies/{id}/match", h.Match)
					r.Put("/shows/{id}/match", h.Match)
					r.Delete("/movies/{id}/match", h.Unmatch)
					r.Delete("/shows/{id}/match", h.Unmatch)
					r.Put("/movies/{id}/ignore", h.Ignore)
					r.Put("/shows/{id}/ignore", h.Ignore)
					r.Post("/media/{id}/detach", h.DetachFile)
					r.Delete("/media/{id}", h.TrashFile)
					r.Get("/shows/{id}/folders", h.ShowFolders)
					r.Post("/shows/{id}/renumber", h.Renumber)
				})

				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole(coremodel.RoleAdmin))
					r.Get("/admin/libraries", h.AdminLibraries)
					r.Post("/admin/libraries", h.CreateLibrary)
					r.Get("/admin/libraries/candidates", h.LibraryCandidates)
					r.Put("/admin/libraries/order", h.ReorderLibraries)
					r.Patch("/admin/libraries/{id}", h.UpdateLibrary)
					r.Delete("/admin/libraries/{id}", h.DeleteLibrary)
					r.Get("/admin/scanner", h.ScannerStatus)
					r.Post("/admin/scanner/rescan", h.Rescan)
					r.Post("/admin/metadata/match-all", h.MatchAll)
					r.Get("/admin/metadata/match", h.MatchStatus)
					r.Post("/admin/metadata/refresh-all", h.RefreshAll)
					r.Get("/admin/metadata/refresh", h.RefreshStatus)
					r.Get("/admin/library/unmatched", h.Unmatched)
					r.Get("/admin/library/show-issues", h.ShowIssues)
					r.Get("/admin/library/movie-issues", h.MovieIssues)
					r.Get("/admin/trash", h.ListTrash)
					r.Post("/admin/trash/restore", h.RestoreTrash)
					r.Post("/admin/trash/delete", h.DeleteTrash)
					r.Post("/admin/trash/empty", h.EmptyTrash)
					r.Get("/admin/stats", h.Stats)
					r.Get("/admin/transcode", h.TranscodeSessions)
					r.Delete("/admin/transcode/{session}", h.StopTranscode)
				})
			})
		})
	})
}

// GET /sceniter/config
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":            h.enabled(),
		"metadata_providers": h.metadataProviders(),
		"transcoding":        h.HLS.Available(),
		"profiles":           []string{"web", "safari"},
	})
}

// userID vrátí přihlášeného uživatele z kontextu (Authenticate).
func userID(r *http.Request) uuid.UUID {
	id, _ := middleware.UserIDFromCtx(r.Context())
	return id
}

// streamUser ověří token ?t= pro přehrávání stejně jako audio Libriteru:
// token se scope stream, existující účet a role aspoň reader.
func (h *Handler) streamUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := h.Auth.ParseStreamToken(r.URL.Query().Get("t"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "auth.stream_token_invalid", "neplatný nebo vypršelý token pro přehrávání")
		return uuid.Nil, false
	}
	role, err := h.Auth.ResolveRole(r.Context(), id)
	if errors.Is(err, coreservice.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "auth.account_gone", "účet již neexistuje")
		return uuid.Nil, false
	}
	if err != nil {
		slog.Error("video: ověření uživatele", "user_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "common.internal", "chyba při ověření uživatele")
		return uuid.Nil, false
	}
	if lvl, ok := coremodel.RoleLevel[role]; !ok || lvl > coremodel.RoleLevel[coremodel.RoleReader] {
		writeError(w, http.StatusForbidden, "auth.forbidden", "nedostatečná oprávnění")
		return uuid.Nil, false
	}
	return id, true
}

// kindParam rozliší film/seriál podle cesty (/movies/… vs /shows/…).
func kindFromPath(r *http.Request) string {
	if strings.Contains(r.URL.Path, "/shows/") {
		return "show"
	}
	return "movie"
}

// writeStoreError odpoví na běžné chyby úložiště; vrací true, pokud odpověděl.
func writeStoreError(w http.ResponseWriter, err error, notFoundCode string) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundCode, "nenalezeno")
	case errors.Is(err, service.ErrInvalid):
		writeError(w, http.StatusBadRequest, "validation.invalid", err.Error())
	default:
		slog.Error("sceniter", "err", err)
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání")
	}
	return true
}

func intQuery(r *http.Request, name string) *int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}
