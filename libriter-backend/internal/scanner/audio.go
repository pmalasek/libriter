// internal/scanner/audio.go
//
// Extrakce metadat z audio souborů.
//
// BookTitle:    Album tag (vyčištěný od číselného prefixu) → název adresáře
// Authors:      AlbumArtist → Composer → Artist → název adresáře → "Neznámý autor"
//               Tag může obsahovat víc autorů oddělených ";", "/", "&", " a ", …;
//               každé jméno se rozdělí na křestní / prostřední / příjmení.
// Narrator:     Artist, pokud se liší od autorů
// ChapterTitle: Title tag → název souboru bez přípony
// TrackNumber:  Track tag → 0 (pořadí z filesystému jako fallback)
// DiscNumber:   Disc tag → 0 (jednodiskové vydání)
// Délka:        ffprobe → 0 (zobrazí varování)

package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"libriter/internal/model"

	"github.com/dhowden/tag"
)

// AudioMeta obsahuje metadata extrahovaná z jednoho audio souboru.
type AudioMeta struct {
	// Metadata na úrovni knihy
	BookTitle string // album tag, vyčištěný od číselných prefixů
	Authors   []model.AuthorName
	Narrator  string // prázdný = neuveden

	// Metadata na úrovni kapitoly
	ChapterTitle    string
	TrackNumber     int // 0 = tag chybí → použije se pořadové číslo
	DiscNumber      int // 0 = tag chybí; >1 = další disk, track čísla začínají znovu od 1
	DurationSeconds int // 0 = ffprobe nedostupný
}

// extractMeta přečte tagy audio souboru a vrátí AudioMeta.
// absPath = absolutní cesta; dirName = název adresáře (fallback pro název knihy/autora).
func extractMeta(absPath, dirName string) (*AudioMeta, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	meta := &AudioMeta{}

	if m, err := tag.ReadFrom(f); err == nil {
		albumArtist := strings.TrimSpace(m.AlbumArtist())
		composer := strings.TrimSpace(m.Composer())
		artist := strings.TrimSpace(m.Artist())

		meta.BookTitle = cleanAlbumTitle(strings.TrimSpace(m.Album()))
		meta.ChapterTitle = strings.TrimSpace(m.Title())

		// Pořadí kapitoly z track čísla, u multi-disk vydání i z čísla disku
		if track, _ := m.Track(); track > 0 {
			meta.TrackNumber = track
		}
		if disc, _ := m.Disc(); disc > 0 {
			meta.DiscNumber = disc
		}

		// Autor: AlbumArtist > Composer > Artist
		authorTag := firstNonEmpty(albumArtist, composer, artist)
		meta.Authors = model.ParseAuthorNames(authorTag)

		// Vypravěč: Artist, pokud jím není jeden z autorů
		if artist != "" && artist != authorTag && !isAuthor(meta.Authors, artist) {
			meta.Narrator = artist
		}
	}

	// Fallbacky
	if meta.BookTitle == "" {
		meta.BookTitle = dirName
	}
	if meta.ChapterTitle == "" {
		base := filepath.Base(absPath)
		meta.ChapterTitle = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if len(meta.Authors) == 0 {
		meta.Authors = model.ParseAuthorNames(dirName)
	}
	if len(meta.Authors) == 0 {
		meta.Authors = []model.AuthorName{model.UnknownAuthor}
	}

	meta.DurationSeconds = ffprobeDuration(absPath)
	return meta, nil
}

// firstNonEmpty vrátí první neprázdnou hodnotu.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// isAuthor zjistí, zda se jméno shoduje s některým z autorů.
func isAuthor(authors []model.AuthorName, name string) bool {
	full := model.ParseAuthorName(name).Full()
	for _, a := range authors {
		if strings.EqualFold(a.Full(), full) {
			return true
		}
	}
	return false
}

// cleanAlbumTitle odstraní číselný prefix jako "01 - " nebo "1. " z názvu alba.
func cleanAlbumTitle(s string) string {
	if idx := strings.Index(s, " - "); idx > 0 {
		prefix := s[:idx]
		allDigits := true
		for _, c := range prefix {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits && len(prefix) <= 3 {
			return strings.TrimSpace(s[idx+3:])
		}
	}
	return s
}

// ffprobeDuration zavolá ffprobe a vrátí délku v sekundách. Při chybě vrátí 0.
func ffprobeDuration(absPath string) int {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_entries", "format=duration",
		absPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &result); err != nil || result.Format.Duration == "" {
		return 0
	}

	secs, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return int(secs)
}
