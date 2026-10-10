package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	"mediater/internal/sceniter/storage"
)

// ErrRenameConflict znamená, že nový název souboru už na disku existuje
// nebo by ho dostaly dva soubory najednou.
var ErrRenameConflict = errors.New("soubor s novým názvem už existuje")

// ErrRenumberInvalid znamená neplatné zadání přečíslování (soubor mimo
// složku nebo seriál, duplicitní nebo neplatné číslo dílu).
var ErrRenumberInvalid = errors.New("neplatné zadání přečíslování")

// RenumberFile je soubor a číslo dílu, které má dostat.
type RenumberFile struct {
	MediaID uuid.UUID
	Episode int
}

// RenumberSubtitle je přejmenování externích titulků vedle videa.
type RenumberSubtitle struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
}

// RenumberChange je jeden soubor plánu: stará a nová cesta a díl, ke
// kterému bude patřit. NewPath == OldPath, když název už číslo nese.
type RenumberChange struct {
	MediaID   uuid.UUID          `json:"media_id"`
	OldPath   string             `json:"old_path"`
	NewPath   string             `json:"new_path"`
	Season    int                `json:"season"`
	Episode   int                `json:"episode"`
	Subtitles []RenumberSubtitle `json:"subtitles"`
}

// RenumberPlan je výsledek přečíslování složky (nebo jeho náhled).
type RenumberPlan struct {
	Season int              `json:"season"`
	Files  []RenumberChange `json:"files"`
}

// PlanRenumber ověří zadání a spočítá nové názvy souborů složky dir
// (relativní k VIDEO_ROOT) seriálu showID, nic nezapisuje. Každý soubor
// dostane v názvu "SxxEyy" (EpisodeFileName) a titulky vedle něj se
// přejmenují s ním.
func (s *Scanner) PlanRenumber(ctx context.Context, showID uuid.UUID, dir string, season int, files []RenumberFile) (*RenumberPlan, error) {
	if season < 0 || len(files) == 0 {
		return nil, ErrRenumberInvalid
	}
	show, err := s.store.GetShow(ctx, showID)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ShowFiles(ctx, showID)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]storage.EpisodeFileRow{}
	for _, r := range rows {
		byID[r.MediaID] = r
	}
	dir = strings.Trim(path.Clean("/"+dir), "/")
	// Díl naplocho v knihovně (složka = složka knihovny) má seriál v názvu
	// souboru; klíč "knihovna/@Název" nese název, který se do něj má vrátit.
	flat := !strings.Contains(dir, "/")
	flatShow := strings.TrimPrefix(path.Base(show.SourceKey), "@")
	plan := &RenumberPlan{Season: season, Files: []RenumberChange{}}
	taken := map[string]bool{}
	episodes := map[int]bool{}
	for _, f := range files {
		r, ok := byID[f.MediaID]
		if !ok || path.Dir(r.FilePath) != dir || f.Episode <= 0 || episodes[f.Episode] {
			return nil, ErrRenumberInvalid
		}
		episodes[f.Episode] = true
		name := path.Base(r.FilePath)
		ext := path.Ext(name)
		oldStem := strings.TrimSuffix(name, ext)
		newStem := EpisodeFileName(oldStem, season, f.Episode)
		if flat {
			newStem = EpisodeFileNameFlat(oldStem, flatShow, season, f.Episode)
		}
		ch := RenumberChange{MediaID: f.MediaID, OldPath: r.FilePath, NewPath: path.Join(dir, newStem+ext), Season: season, Episode: f.Episode, Subtitles: []RenumberSubtitle{}}
		if ch.NewPath != ch.OldPath {
			if taken[ch.NewPath] {
				return nil, fmt.Errorf("%w: %s", ErrRenameConflict, ch.NewPath)
			}
			if _, err := os.Lstat(s.abs(ch.NewPath)); err == nil {
				return nil, fmt.Errorf("%w: %s", ErrRenameConflict, ch.NewPath)
			}
			taken[ch.NewPath] = true
			for _, sub := range s.subtitleNames(ch.OldPath) {
				rest := sub[len(oldStem):]
				ns := path.Join(dir, newStem+rest)
				if taken[ns] {
					return nil, fmt.Errorf("%w: %s", ErrRenameConflict, ns)
				}
				if _, err := os.Lstat(s.abs(ns)); err == nil {
					return nil, fmt.Errorf("%w: %s", ErrRenameConflict, ns)
				}
				taken[ns] = true
				ch.Subtitles = append(ch.Subtitles, RenumberSubtitle{OldPath: path.Join(dir, sub), NewPath: ns})
			}
		}
		plan.Files = append(plan.Files, ch)
	}
	return plan, nil
}

// Renumber provede PlanRenumber: soubory přeřadí k dílům a přejmenuje na
// disku. Pro každý soubor nejdřív zapíše databázi a pak přejmenuje – sledování
// změn tak starou cestu v databázi už nenajde a nic nesmaže, a novou cestu
// najde jako známý soubor. Když přejmenování selže, zápis toho souboru se
// vrátí; soubory před ním zůstávají hotové.
func (s *Scanner) Renumber(ctx context.Context, showID uuid.UUID, dir string, season int, files []RenumberFile) (*RenumberPlan, error) {
	s.ingest.Lock()
	defer s.ingest.Unlock()

	plan, err := s.PlanRenumber(ctx, showID, dir, season, files)
	if err != nil {
		return nil, err
	}
	show, err := s.store.GetShow(ctx, showID)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ShowFiles(ctx, showID)
	if err != nil {
		return nil, err
	}
	current := map[uuid.UUID]storage.EpisodeFileRow{}
	for _, r := range rows {
		current[r.MediaID] = r
	}
	year := 0
	if show.Year != nil {
		year = *show.Year
	}
	for i, ch := range plan.Files {
		cur := current[ch.MediaID]
		newStem := strings.TrimSuffix(path.Base(ch.NewPath), path.Ext(ch.NewPath))
		key := storage.EpisodeKey{
			LibraryID: show.LibraryID, ShowKey: show.SourceKey, ShowTitle: show.Title, ShowYear: year,
			Season: ch.Season, Episode: ch.Episode, Title: ParseEpisode(newStem).Title,
		}
		episodeID := cur.EpisodeID
		if cur.Season != ch.Season || cur.Episode != ch.Episode {
			if episodeID, err = s.store.EnsureEpisode(ctx, key); err != nil {
				return nil, s.renumberError(i, plan, err)
			}
		}
		if ch.NewPath == ch.OldPath {
			if episodeID != cur.EpisodeID {
				if err := s.store.RenameMedia(ctx, ch.MediaID, ch.NewPath, episodeID, nil); err != nil {
					return nil, s.renumberError(i, plan, err)
				}
			}
			continue
		}
		subs := map[string]string{}
		for _, sub := range ch.Subtitles {
			subs[sub.OldPath] = sub.NewPath
		}
		if err := s.store.RenameMedia(ctx, ch.MediaID, ch.NewPath, episodeID, subs); err != nil {
			return nil, s.renumberError(i, plan, err)
		}
		if err := os.Rename(s.abs(ch.OldPath), s.abs(ch.NewPath)); err != nil {
			back := map[string]string{}
			for old, new := range subs {
				back[new] = old
			}
			if rerr := s.store.RenameMedia(ctx, ch.MediaID, ch.OldPath, cur.EpisodeID, back); rerr != nil {
				s.log.Error("vrácení přejmenování v databázi", "path", ch.OldPath, "err", rerr)
			}
			return nil, s.renumberError(i, plan, fmt.Errorf("přejmenování %s: %w", ch.OldPath, err))
		}
		for _, sub := range ch.Subtitles {
			if err := os.Rename(s.abs(sub.OldPath), s.abs(sub.NewPath)); err != nil {
				// Titulky zůstanou pod starým názvem; sledování změn je odpáruje.
				s.log.Warn("přejmenování titulků", "path", sub.OldPath, "err", err)
			}
		}
		s.log.Info("díl přečíslován", "z", ch.OldPath, "na", ch.NewPath,
			"díl", fmt.Sprintf("S%02dE%02d", ch.Season, ch.Episode))
	}
	return plan, nil
}

func (s *Scanner) renumberError(done int, plan *RenumberPlan, err error) error {
	return fmt.Errorf("přečíslováno %d z %d souborů: %w", done, len(plan.Files), err)
}

func (s *Scanner) abs(rel string) string {
	return filepath.Join(s.root, filepath.FromSlash(rel))
}

// subtitleNames vrátí názvy souborů titulků vedle videa rel (bez složky),
// seřazené podle názvu.
func (s *Scanner) subtitleNames(rel string) []string {
	abs := s.abs(rel)
	stem := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	entries, err := os.ReadDir(filepath.Dir(abs))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || Ignored(e.Name()) {
			continue
		}
		if _, _, ok := SubtitleMatch(stem, e.Name()); ok {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}
