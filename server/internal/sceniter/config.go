package sceniter

import (
	"os"
	"path/filepath"

	"mediater/internal/core/config"
)

// Config je konfigurace Sceniteru. Jádro o ní neví – produkt si ji čte sám
// z prostředí (a .env) přes pomocníky jádra.
type Config struct {
	// VideoRoot je kořen videoknihovny. Prázdný = Sceniter je vypnutý:
	// API vrací sceniter.disabled a scanner neběží. Které podsložky se
	// procházejí a jako co, nastavuje admin v knihovnách (sc_libraries).
	VideoRoot string
	// ImageRoot je adresář plakátů a obrázků ze zdrojů metadat.
	ImageRoot string
	// TranscodeRoot je pracovní adresář HLS převodů; obsah je dočasný.
	TranscodeRoot string
	// TranscodeMaxSessions omezuje souběžné převody (každý vytíží CPU).
	TranscodeMaxSessions int
	// TranscodeIdleSeconds: převod, na který nikdo nesáhl, se po této době ukončí.
	TranscodeIdleSeconds int
	// TranscodeMaxHeight je strop výšky obrazu při plném převodu.
	TranscodeMaxHeight int
	// RescanMinutes > 0 zapne pravidelný průchod knihovnou (síťové disky, na
	// kterých fsnotify nefunguje).
	RescanMinutes int
}

// LoadConfig načte konfiguraci Sceniteru.
func LoadConfig(c *config.Config) Config {
	cfg := Config{
		VideoRoot:            c.EnvPath("VIDEO_ROOT", ""),
		ImageRoot:            c.EnvPath("SCENITER_IMAGE_ROOT", ""),
		TranscodeRoot:        c.EnvPath("TRANSCODE_ROOT", ""),
		TranscodeMaxSessions: c.EnvInt("TRANSCODE_MAX_SESSIONS", 2),
		TranscodeIdleSeconds: c.EnvInt("TRANSCODE_IDLE_SECONDS", 60),
		TranscodeMaxHeight:   c.EnvInt("TRANSCODE_MAX_HEIGHT", 1080),
		RescanMinutes:        c.EnvInt("SCENITER_RESCAN_MINUTES", 0),
	}
	// Obrázky vedle obálek knih – kdo nastaví jen COVER_ROOT, chce data pohromadě.
	if cfg.ImageRoot == "" {
		cfg.ImageRoot = filepath.Join(filepath.Dir(c.Storage.CoverRoot), "sceniter-images")
	}
	if cfg.TranscodeRoot == "" {
		cfg.TranscodeRoot = filepath.Join(os.TempDir(), "mediater-transcode")
	}
	if cfg.TranscodeMaxSessions < 1 {
		cfg.TranscodeMaxSessions = 1
	}
	if cfg.TranscodeIdleSeconds < 10 {
		cfg.TranscodeIdleSeconds = 10
	}
	if cfg.TranscodeMaxHeight < 240 {
		cfg.TranscodeMaxHeight = 1080
	}
	return cfg
}

// Enabled řekne, jestli je videoknihovna nastavená.
func (c Config) Enabled() bool { return c.VideoRoot != "" }
