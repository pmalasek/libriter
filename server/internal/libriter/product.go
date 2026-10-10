// Package libriter zapojuje knihovnu audioknih do serveru jako produkt.
//
// API Libriteru zůstává kvůli vydané mobilní aplikaci na kořeni /api/v1
// (bez prefixu produktu) – cesty ani tvary odpovědí se nesmí měnit.
package libriter

import (
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/middleware"
	"mediater/internal/core/model"
	"mediater/internal/core/product"
	"mediater/internal/libriter/handler"
	"mediater/internal/libriter/importer"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/scanner"
	"mediater/internal/libriter/service"
	"mediater/internal/libriter/storage"
	"mediater/internal/libriter/web"
)

// Product je Libriter – knihovna a přehrávač audioknih.
type Product struct {
	deps product.Deps

	bookH     *handler.BookHandler
	authorH   *handler.AuthorHandler
	seriesH   *handler.SeriesHandler
	metadataH *handler.MetadataHandler
	adminH    *handler.AdminHandler
	sessionH  *handler.PlaySessionHandler
	audioH    *handler.AudioHandler
	importH   *handler.ImportHandler
}

func New() *Product { return &Product{} }

func (p *Product) Name() string { return "libriter" }

// Migrations: schéma Libriteru je zatím celé v základní řadě 001–017 jádra
// (vzniklo dřív než rozdělení na produkty). Nové knižní migrace patří sem.
func (p *Product) Migrations() fs.FS { return nil }

func (p *Product) Web() fs.FS { return web.FS() }

func (p *Product) Init(d product.Deps) error {
	p.deps = d
	cfg := d.Cfg
	ctx := d.Ctx

	store := storage.New(d.DB)

	// --- scanner ---
	// Spouští se až po načtení nastavení – nové knihy dostávají výchozí jazyk
	// knihovny a počáteční průchod by je jinak zakládal se záložním.
	scn := scanner.New(cfg.Storage.AudioRoot, cfg.Storage.CoverRoot, store)

	// Zdroje metadat musí vzniknout dřív než služby, které je používají –
	// stahování fotek autorů si přes ně ověřuje povolené adresy. Registry
	// drží aktuální řetězec a mění ho, když admin přenastaví zdroje; hodnoty
	// z .env jsou jen výchozí, uložené nastavení má přednost.
	//
	// Zdroje jsou pluginy (core/plugin); seznam se mění, když admin pluginy
	// znovu načte.
	registry := metadata.NewRegistry(metadata.PluginFactories(d.Plugins))
	settingsSvc := service.NewSettingsFunc(store, cfg.Metadata, cfg.Library, registry.KnownNames)
	if d.PluginSettings != nil {
		settingsSvc.WithGoogleKeyStore(d.PluginSettings)
		if err := settingsSvc.MigrateGoogleKey(ctx); err != nil {
			return fmt.Errorf("klíč Google Books: %w", err)
		}
	}

	rebuild := func() error {
		profiles, googleKey, err := settingsSvc.Profiles(ctx)
		if err != nil {
			return fmt.Errorf("nastavení zdrojů metadat: %w", err)
		}
		registry.Rebuild(profiles, metadata.ProviderConfig{GoogleBooksAPIKey: googleKey})
		return nil
	}
	if err := rebuild(); err != nil {
		return err
	}
	if d.Plugins != nil {
		d.Plugins.OnChange(func() {
			registry.SetFactories(metadata.PluginFactories(d.Plugins))
			if err := rebuild(); err != nil {
				slog.Warn("zdroje metadat po načtení pluginů", "err", err)
			}
		})
	}

	scn.SetDefaultLanguage(settingsSvc.DefaultLanguage)
	scn.Start(ctx)

	bookSvc := service.NewBook(store, scn)
	authorSvc := service.NewAuthor(store)
	authorImageSvc := service.NewAuthorImage(store, registry, cfg.Storage.AuthorImageRoot)
	seriesSvc := service.NewSeries(store)
	systemSvc := service.NewSystem(store, cfg, d.StartedAt)
	playSessionSvc := service.NewPlaySession(store)
	listeningSvc := service.NewListening(store)

	p.bookH = handler.NewBook(bookSvc, cfg.Storage.CoverRoot, d.Audit).
		WithCovers(service.NewBookCover(store, registry, cfg.Storage.CoverRoot))
	p.authorH = handler.NewAuthor(authorSvc, cfg.Storage.AuthorImageRoot, authorImageSvc, d.Audit)
	p.seriesH = handler.NewSeries(seriesSvc, d.Audit)
	p.metadataH = handler.NewMetadata(registry)
	p.adminH = handler.NewAdmin(d.Users, settingsSvc, registry, scn, systemSvc, d.Audit, listeningSvc)
	p.sessionH = handler.NewPlaySession(playSessionSvc)
	p.audioH = handler.NewAudio(bookSvc, d.Auth, cfg.Storage.AudioRoot)

	importSvc, err := importer.New(ctx, cfg.Storage.ImportRoot,
		cfg.Storage.MaxUploadMB<<20, scn, store)
	if err != nil {
		return fmt.Errorf("import knih (IMPORT_ROOT=%s): %w", cfg.Storage.ImportRoot, err)
	}
	p.importH = handler.NewImport(importSvc, d.Audit)
	return nil
}

func (p *Product) RegisterRoutes(r chi.Router) {
	bookH, authorH, seriesH := p.bookH, p.authorH, p.seriesH
	metadataH, adminH, sessionH := p.metadataH, p.adminH, p.sessionH
	audioH, importH := p.audioH, p.importH

	// --- obálky (bez přihlášení) ---
	// <img> neumí poslat hlavičku Authorization; ochranou je neuhodnutelné UUID knihy.
	r.Get("/books/{id}/cover", bookH.Cover)
	r.Head("/books/{id}/cover", bookH.Cover)
	r.Get("/authors/{id}/image", authorH.Image)
	r.Head("/authors/{id}/image", authorH.Image)

	// --- audio kapitoly (mimo Authenticate) ---
	// <audio> neumí poslat hlavičku Authorization, takže se ověřuje
	// krátkodobým tokenem v adrese (?t=). Není to přihlašovací token –
	// platí jen na streamování a vydává ho /auth/stream-token.
	r.Get("/chapters/{id}/audio", audioH.Stream)
	r.Head("/chapters/{id}/audio", audioH.Stream)

	// --- chráněné endpointy ---
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(p.deps.Auth))

		// Knihy / autoři / série - čtení (reader+)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleReader))
			r.Get("/books", bookH.List)
			// Stav knih musí být nad /books/{id}, ať se "progress"
			// nečte jako UUID knihy.
			r.Get("/books/progress", bookH.ListProgress)
			r.Get("/books/{id}", bookH.Get)
			r.Put("/books/{id}/progress", bookH.SetProgress)
			r.Delete("/books/{id}/progress", bookH.ResetProgress)
			r.Get("/books/{id}/chapters", bookH.ListChapters)
			r.Get("/authors", authorH.List)
			r.Get("/authors/{id}", authorH.Get)
			r.Get("/series", seriesH.List)
			r.Get("/languages", bookH.Languages)
			r.Get("/series/{id}", seriesH.Get)

			// Poslechové session – vlastní data přihlášeného uživatele
			r.Get("/sessions", sessionH.List)
			r.Post("/sessions", sessionH.Create)
			// Dávka pozic z offline zařízení; musí být nad /sessions/{id},
			// ať se "sync" nečte jako UUID session.
			r.Post("/sessions/sync", sessionH.Sync)
			r.Get("/sessions/{id}", sessionH.Get)
			r.Put("/sessions/{id}/position", sessionH.SavePosition)
			r.Post("/sessions/{id}/items", sessionH.AddItems)
			r.Delete("/sessions/{id}", sessionH.Delete)
		})

		// Knihy / autoři / série - zápis (editor+)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleEditor))
			r.Post("/books", bookH.Create)
			r.Put("/books/{id}", bookH.Update)
			r.Patch("/books/{id}", bookH.Patch) // částečná aktualizace (webové rozhraní)
			r.Put("/books/{id}/chapters/order", bookH.ReorderChapters)
			r.Put("/books/{id}/cover", bookH.SetCover)     // stáhne obálku ze zdroje
			r.Post("/books/{id}/cover", bookH.UploadCover) // nahraný soubor
			r.Post("/authors", authorH.Create)
			r.Put("/authors/{id}", authorH.Update)
			r.Put("/authors/{id}/image", authorH.SetImage)     // stáhne fotku ze zdroje
			r.Post("/authors/{id}/image", authorH.UploadImage) // nahraný soubor
			r.Delete("/authors/{id}/image", authorH.DeleteImage)
			r.Post("/series", seriesH.Create)
			r.Put("/series/{id}", seriesH.Update)

			// Metadata knih - zdroje a jejich pořadí řídí administrace
			r.Get("/metadata/sources", metadataH.Sources) // pořadí zdrojů
			r.Get("/metadata/search", metadataH.Search)   // ?q=<dotaz>
			r.Get("/metadata/author/search", metadataH.SearchAuthors)
			r.Get("/metadata/author", metadataH.FetchAuthorByURL) // ?url=<url>
			r.Get("/metadata/book/{id}", metadataH.FetchByID)     // dle DK ID
			r.Get("/metadata/book", metadataH.FetchByURL)         // ?url=<url>
		})

		// Mazání a administrace knihovny (admin). Plné cesty místo
		// r.Route("/admin"): /admin/users a spol. registruje jádro.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleAdmin))
			r.Delete("/books/{id}", bookH.Delete)
			r.Delete("/authors/{id}", authorH.Delete)
			r.Delete("/series/{id}", seriesH.Delete)

			r.Get("/admin/settings/metadata", adminH.MetadataSettings)
			r.Put("/admin/settings/metadata", adminH.SetMetadataSettings)
			r.Get("/admin/settings/library", adminH.LibrarySettings)
			r.Put("/admin/settings/library", adminH.SetLibrarySettings)

			r.Get("/admin/scanner", adminH.ScannerStatus)
			r.Post("/admin/scanner/rescan", adminH.Rescan)
			r.Get("/admin/library/repair", adminH.RepairPlan) // náhled, nic nemění
			r.Post("/admin/library/repair", adminH.Repair)
			r.Get("/admin/library/duplicates", adminH.DuplicateReport) // jen hlášení
			r.Post("/admin/library/duplicates/dismiss", adminH.DismissDuplicate)
			r.Delete("/admin/library/duplicates/dismiss", adminH.RestoreDuplicate)
			r.Get("/admin/library/merge", adminH.MergePlan) // náhled, nic nemění
			r.Post("/admin/library/merge", adminH.Merge)

			r.Get("/admin/import", importH.List)
			r.Post("/admin/import", importH.Create)
			r.Get("/admin/import/{id}", importH.Get)
			r.Delete("/admin/import/{id}", importH.Delete)
			r.Get("/admin/import/{id}/files", importH.Files)
			r.Put("/admin/import/{id}/files", importH.Upload) // ?path=<relativní cesta>
			r.Post("/admin/import/{id}/analyze", importH.Analyze)
			r.Post("/admin/import/{id}/commit", importH.Commit)
			r.Get("/admin/import/{id}/cover", importH.Cover) // ?key=<adresář knihy>

			r.Get("/admin/stats", adminH.Stats)
			r.Get("/admin/system", adminH.System)

			r.Get("/admin/listening", adminH.Listening)
			r.Get("/admin/listening/{id}", adminH.ListeningUser)
		})
	})
}
