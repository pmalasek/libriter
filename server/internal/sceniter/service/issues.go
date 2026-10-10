package service

import (
	"context"
	"path"
	"sort"
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
	// FileIDs jsou ID souborů z Files (IssueDuplicateFiles) – pro smazání z kontroly.
	FileIDs []uuid.UUID `json:"file_ids,omitempty"`
	// FileSizes jsou velikosti souborů z Files v bajtech (IssueDuplicateFiles).
	FileSizes []int64 `json:"file_sizes,omitempty"`
	// Dir je složka dotčených souborů (u IssueGap prvního souboru řady) –
	// odkaz na editor složky, kde se číslování opraví.
	Dir string `json:"dir,omitempty"`
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
			var ids []uuid.UUID
			var sizes []int64
			for k < len(files) && files[k].Season == season && files[k].Episode == files[j].Episode {
				paths = append(paths, files[k].FilePath)
				ids = append(ids, files[k].MediaID)
				sizes = append(sizes, files[k].SizeBytes)
				if !scanner.EpisodeFromPath(files[k].FilePath, files[k].EmbeddedTitle).OK {
					positional = append(positional, files[k].FilePath)
				}
				k++
			}
			number := files[j].Episode
			numbers = append(numbers, number)
			if len(paths) > 1 {
				out = append(out, ShowIssueItem{Kind: IssueDuplicateFiles, Season: season, Episode: number, Files: paths, FileIDs: ids, FileSizes: sizes, Dir: path.Dir(paths[0])})
			}
			j = k
		}
		if len(positional) > 0 {
			out = append(out, ShowIssueItem{Kind: IssuePositional, Season: season, Files: positional, Dir: path.Dir(positional[0])})
		}
		// Speciály (řada 0) se číslují volně, díry v nich nic neznamenají.
		if season > 0 {
			if missing := gaps(numbers); len(missing) > 0 {
				out = append(out, ShowIssueItem{Kind: IssueGap, Season: season, Missing: missing, Dir: path.Dir(files[i].FilePath)})
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

// MovieIssueFile je soubor filmu s více soubory. Detachable říká, že soubor
// se k filmu přisloučil podle metadat a lze ho oddělit (scanner.DetachFile).
type MovieIssueFile struct {
	ID         uuid.UUID `json:"id"`
	FilePath   string    `json:"file_path"`
	SizeBytes  int64     `json:"size_bytes"`
	Detachable bool      `json:"detachable"`
}

// MovieIssue je film, ke kterému se přisloučil soubor jiného filmu.
type MovieIssue struct {
	ID         uuid.UUID        `json:"id"`
	Title      string           `json:"title"`
	SourceKey  string           `json:"source_key"`
	PosterPath *string          `json:"poster_path,omitempty"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Files      []MovieIssueFile `json:"files"`
}

// MovieIssues vrátí filmy s více soubory, z nichž aspoň jeden nepatří
// k filmu podle vlastní cesty (přisloučil se podle metadat). Film rozdělený
// do více částí (cd1, cd2) má u všech souborů stejný klíč a nehlásí se.
func (l *Library) MovieIssues(ctx context.Context) ([]MovieIssue, error) {
	rows, err := l.store.MovieFiles(ctx)
	if err != nil {
		return nil, err
	}
	out := []MovieIssue{}
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].MovieID == rows[i].MovieID {
			j++
		}
		r := rows[i]
		m := MovieIssue{ID: r.MovieID, Title: r.Title, SourceKey: r.SourceKey, PosterPath: r.PosterPath, UpdatedAt: r.UpdatedAt, Files: []MovieIssueFile{}}
		foreign := false
		for _, f := range rows[i:j] {
			detachable := scanner.MovieKeyOf(f.FilePath) != r.SourceKey
			foreign = foreign || detachable
			m.Files = append(m.Files, MovieIssueFile{ID: f.MediaID, FilePath: f.FilePath, SizeBytes: f.SizeBytes, Detachable: detachable})
		}
		if foreign {
			out = append(out, m)
		}
		i = j
	}
	return out, nil
}

// ShowFolderFile je soubor dílu v editoru složky.
type ShowFolderFile struct {
	ID       uuid.UUID `json:"id"`
	FilePath string    `json:"file_path"`
	Name     string    `json:"name"`
	Season   int       `json:"season"`
	Episode  int       `json:"episode"`
	// FromName: číslo dílu nese název souboru; jinak ho určilo pořadí ve složce.
	FromName bool `json:"from_name"`
}

// ShowFolder je složka se soubory seriálu. Season je řada podle názvu
// složky, jinak řada většiny souborů v ní.
type ShowFolder struct {
	Dir    string           `json:"dir"`
	Season int              `json:"season"`
	Files  []ShowFolderFile `json:"files"`
}

// ShowFolders vrátí složky seriálu se soubory dílů (řazené podle řady,
// dílu a cesty), složky podle cesty.
func (l *Library) ShowFolders(ctx context.Context, showID uuid.UUID) ([]ShowFolder, error) {
	show, err := l.store.GetShow(ctx, showID)
	if err != nil {
		return nil, err
	}
	rows, err := l.store.ShowFiles(ctx, showID)
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	out := []ShowFolder{}
	for _, r := range rows {
		dir := path.Dir(r.FilePath)
		i, ok := index[dir]
		if !ok {
			i = len(out)
			index[dir] = i
			out = append(out, ShowFolder{Dir: dir, Files: []ShowFolderFile{}})
		}
		out[i].Files = append(out[i].Files, ShowFolderFile{
			ID: r.MediaID, FilePath: r.FilePath, Name: path.Base(r.FilePath), Season: r.Season, Episode: r.Episode,
			FromName: scanner.EpisodeFromPath(r.FilePath, r.EmbeddedTitle).OK,
		})
	}
	for i := range out {
		out[i].Season = folderSeason(out[i], show.Title)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Dir < out[b].Dir })
	return out, nil
}

// folderSeason určí řadu složky: z jejího názvu, jinak nejčastější řada
// souborů v ní (při shodě nižší).
func folderSeason(f ShowFolder, show string) int {
	if n, ok := scanner.ParseSeasonDirIn(path.Base(f.Dir), show); ok {
		return n
	}
	counts := map[int]int{}
	best := 1
	for _, file := range f.Files {
		counts[file.Season]++
		if counts[file.Season] > counts[best] || (counts[file.Season] == counts[best] && file.Season < best) {
			best = file.Season
		}
	}
	return best
}
