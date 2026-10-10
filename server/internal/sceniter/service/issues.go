package service

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"mediater/internal/sceniter/scanner"
	"mediater/internal/sceniter/storage"
)

// Druhy nálezů kontroly seriálů.
const (
	// IssueDuplicateFiles: díl má víc souborů (často chybně očíslovaný díl).
	IssueDuplicateFiles = "duplicate_files"
	// IssueGap: v řadě chybí díly mezi prvním a posledním na disku.
	IssueGap = "gap"
	// IssuePositional: číslo dílu není v názvu souboru, určilo ho pořadí v adresáři.
	IssuePositional = "positional"
)

// ShowIssueItem je jeden nález u seriálu.
type ShowIssueItem struct {
	Kind    string `json:"kind"`
	Season  int    `json:"season"`
	Episode int    `json:"episode,omitempty"`
	// Missing jsou chybějící čísla dílů (IssueGap).
	Missing []int `json:"missing,omitempty"`
	// Files jsou dotčené soubory (IssueDuplicateFiles, IssuePositional –
	// u něj všechny takové soubory řady).
	Files []string `json:"files,omitempty"`
}

// ShowIssue je seriál s nálezy kontroly číslování dílů.
type ShowIssue struct {
	ID         uuid.UUID       `json:"id"`
	Title      string          `json:"title"`
	SourceKey  string          `json:"source_key"`
	PosterPath *string         `json:"poster_path,omitempty"`
	UpdatedAt  time.Time       `json:"updated_at"`
	Issues     []ShowIssueItem `json:"issues"`
}

// ShowIssues projde soubory dílů všech seriálů a vrátí seriály, u kterých
// číslování nesedí: díl s více soubory, díra v číslování řady, nebo díl
// očíslovaný jen podle pořadí v adresáři. Seriály bez nálezů vynechá.
func (l *Library) ShowIssues(ctx context.Context) ([]ShowIssue, error) {
	rows, err := l.store.EpisodeFiles(ctx)
	if err != nil {
		return nil, err
	}
	out := []ShowIssue{}
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].ShowID == rows[i].ShowID {
			j++
		}
		r := rows[i]
		sh := ShowIssue{ID: r.ShowID, Title: r.ShowTitle, SourceKey: r.SourceKey, PosterPath: r.PosterPath, UpdatedAt: r.UpdatedAt}
		sh.Issues = showIssues(rows[i:j])
		if len(sh.Issues) > 0 {
			out = append(out, sh)
		}
		i = j
	}
	return out, nil
}

// showIssues vyhodnotí soubory jednoho seriálu (seřazené podle řady a dílu).
func showIssues(files []storage.EpisodeFileRow) []ShowIssueItem {
	var out []ShowIssueItem
	for i := 0; i < len(files); {
		season := files[i].Season
		j := i
		var numbers []int
		var positional []string
		for j < len(files) && files[j].Season == season {
			k := j
			var paths []string
			for k < len(files) && files[k].Season == season && files[k].Episode == files[j].Episode {
				paths = append(paths, files[k].FilePath)
				k++
			}
			number := files[j].Episode
			numbers = append(numbers, number)
			if len(paths) > 1 {
				out = append(out, ShowIssueItem{Kind: IssueDuplicateFiles, Season: season, Episode: number, Files: paths})
			}
			for _, p := range paths {
				stem := strings.TrimSuffix(path.Base(p), path.Ext(p))
				if !scanner.ParseEpisode(stem).OK {
					positional = append(positional, p)
				}
			}
			j = k
		}
		if len(positional) > 0 {
			out = append(out, ShowIssueItem{Kind: IssuePositional, Season: season, Files: positional})
		}
		// Speciály (řada 0) se číslují volně, díry v nich nic neznamenají.
		if season > 0 {
			if missing := gaps(numbers); len(missing) > 0 {
				out = append(out, ShowIssueItem{Kind: IssueGap, Season: season, Missing: missing})
			}
		}
		i = j
	}
	return out
}

// gaps vrátí čísla chybějící mezi 1 a nejvyšším číslem (vstup je seřazený).
func gaps(numbers []int) []int {
	var missing []int
	next := 1
	for _, n := range numbers {
		for ; next < n; next++ {
			missing = append(missing, next)
		}
		if n >= next {
			next = n + 1
		}
	}
	return missing
}
