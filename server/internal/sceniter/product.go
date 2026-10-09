// Package sceniter zapojuje videoknihovnu (filmy a seriály) do serveru jako
// produkt. API je pod /api/v1/sceniter, schéma má vlastní řadu migrací a
// tabulky s prefixem sc_. Účty, přihlášení a role jsou společné s Libriterem.
package sceniter

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/product"
	"mediater/internal/sceniter/handler"
	"mediater/internal/sceniter/migrations"
	"mediater/internal/sceniter/scanner"
	"mediater/internal/sceniter/service"
	"mediater/internal/sceniter/storage"
	"mediater/internal/sceniter/stream"
	"mediater/internal/sceniter/tmdb"
	"mediater/internal/sceniter/web"
)

// Product je Sceniter.
type Product struct {
	cfg Config
	h   *handler.Handler
}

func New() *Product { return &Product{} }

func (p *Product) Name() string      { return "sceniter" }
func (p *Product) Migrations() fs.FS { return migrations.FS }
func (p *Product) Web() fs.FS        { return web.FS() }

func (p *Product) Init(d product.Deps) error {
	p.cfg = LoadConfig(d.Cfg)
	cfg := p.cfg

	store := storage.New(d.DB)
	client := tmdb.New(cfg.TMDBAPIKey, cfg.TMDBLanguage)
	matcher := service.NewMatcher(store, client, cfg.ImageRoot)
	matcher.Start(d.Ctx)

	hls := stream.NewManager(cfg.TranscodeRoot, cfg.TranscodeMaxSessions, time.Duration(cfg.TranscodeIdleSeconds)*time.Second)
	if err := os.MkdirAll(cfg.TranscodeRoot, 0o755); err != nil {
		slog.Warn("TRANSCODE_ROOT nejde vytvořit – převody videa nepůjdou", "path", cfg.TranscodeRoot, "err", err)
	}
	hls.Start(d.Ctx)

	deps := handler.Deps{
		Store: store, Library: service.NewLibrary(store), Matcher: matcher, HLS: hls,
		Subs: stream.NewSubtitles(filepath.Join(cfg.TranscodeRoot, "subs")),
		Auth: d.Auth, Audit: d.Audit,
		VideoRoot: cfg.VideoRoot, ImageRoot: cfg.ImageRoot, MaxHeight: cfg.TranscodeMaxHeight,
	}

	if cfg.Enabled() {
		if err := os.MkdirAll(cfg.ImageRoot, 0o755); err != nil {
			slog.Warn("SCENITER_IMAGE_ROOT nejde vytvořit", "path", cfg.ImageRoot, "err", err)
		}
		scn := scanner.New(cfg.VideoRoot, cfg.MovieDirs, cfg.ShowDirs, store)
		scn.OnChange = matcher.Kick
		scn.Start(d.Ctx, time.Duration(cfg.RescanMinutes)*time.Minute)
		deps.Scanner = scn
		slog.Info("sceniter", "video_root", cfg.VideoRoot, "tmdb", client.Enabled(), "ffmpeg", hls.Available())
	} else {
		slog.Info("sceniter vypnutý – VIDEO_ROOT není nastaven")
	}

	p.h = handler.New(deps)
	return nil
}

func (p *Product) RegisterRoutes(r chi.Router) { p.h.Routes(r) }
