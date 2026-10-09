package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// ErrUnsupportedSubtitle znamená titulky, které nejde převést do WebVTT.
var ErrUnsupportedSubtitle = errors.New("titulky nejde převést do WebVTT")

// Subtitles převádí titulky do WebVTT a výsledek si pamatuje na disku.
type Subtitles struct {
	ffmpeg string
	cache  string // adresář mezipaměti (TRANSCODE_ROOT/subs)
}

func NewSubtitles(cacheDir string) *Subtitles {
	ffmpeg, _ := exec.LookPath("ffmpeg")
	return &Subtitles{ffmpeg: ffmpeg, cache: cacheDir}
}

// Embedded vrátí vloženou titulkovou stopu idx souboru abs jako WebVTT.
func (s *Subtitles) Embedded(ctx context.Context, cacheKey, abs string, idx int) ([]byte, error) {
	if s.ffmpeg == "" {
		return nil, ErrNoFFmpeg
	}
	return s.cached(cacheKey, func() ([]byte, error) {
		return s.ffmpegVTT(ctx, abs, "0:"+strconv.Itoa(idx))
	})
}

// External vrátí externí soubor titulků jako WebVTT. SRT se převádí v Go
// (s rozpoznáním kódování – české titulky bývají ve Windows-1250), ASS/SSA
// přes ffmpeg, VTT se jen ověří.
func (s *Subtitles) External(ctx context.Context, cacheKey, abs, format string) ([]byte, error) {
	return s.cached(cacheKey, func() ([]byte, error) {
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, err
		}
		switch format {
		case "srt":
			return SRTToVTT(DecodeText(data)), nil
		case "vtt":
			text := DecodeText(data)
			if !strings.HasPrefix(strings.TrimPrefix(text, "\ufeff"), "WEBVTT") {
				text = "WEBVTT\n\n" + text
			}
			return []byte(text), nil
		case "ass":
			if s.ffmpeg == "" {
				return nil, ErrNoFFmpeg
			}
			// ffmpeg čte ASS jako UTF-8; soubor ve Windows-1250 se nejdřív převede.
			tmp, err := os.CreateTemp("", "sub-*.ass")
			if err != nil {
				return nil, err
			}
			defer os.Remove(tmp.Name())
			if _, err := tmp.WriteString(DecodeText(data)); err != nil {
				tmp.Close()
				return nil, err
			}
			tmp.Close()
			return s.ffmpegVTT(ctx, tmp.Name(), "0:s:0")
		}
		return nil, ErrUnsupportedSubtitle
	})
}

func (s *Subtitles) ffmpegVTT(ctx context.Context, abs, mapSpec string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", abs, "-map", mapSpec, "-f", "webvtt", "pipe:1")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSubtitle, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

func (s *Subtitles) cached(key string, produce func() ([]byte, error)) ([]byte, error) {
	var path string
	if s.cache != "" && key != "" && key == filepath.Base(key) {
		path = filepath.Join(s.cache, key+".vtt")
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}
	data, err := produce()
	if err != nil {
		return nil, err
	}
	if path != "" {
		if err := os.MkdirAll(s.cache, 0o755); err == nil {
			_ = os.WriteFile(path, data, 0o644)
		}
	}
	return data, nil
}

// DecodeText převede text titulků do UTF-8. Platné UTF-8 (i s BOM) se
// nechá, jinak se bere Windows-1250 – nejčastější kódování českých a
// slovenských titulků. Latin-1 by češtinu rozbil, cp1250 západní texty
// zobrazí skoro správně.
func DecodeText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if utf8.Valid(data) {
		return string(data)
	}
	out, err := charmap.Windows1250.NewDecoder().Bytes(data)
	if err != nil {
		return string(data)
	}
	return string(out)
}

var srtTimeRe = regexp.MustCompile(`(\d{1,2}:\d{2}:\d{2})[,.](\d{1,3})`)

// SRTToVTT převede titulky SubRip do WebVTT: hlavička, tečka místo čárky
// v časech, bez číselných indexů bloků.
func SRTToVTT(text string) []byte {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, block := range strings.Split(strings.TrimSpace(text), "\n\n") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) > 0 && !strings.Contains(lines[0], "-->") {
			if _, err := strconv.Atoi(strings.TrimSpace(lines[0])); err == nil {
				lines = lines[1:]
			}
		}
		if len(lines) == 0 || !strings.Contains(lines[0], "-->") {
			continue
		}
		lines[0] = srtTimeRe.ReplaceAllStringFunc(lines[0], func(m string) string {
			p := srtTimeRe.FindStringSubmatch(m)
			ms := p[2]
			for len(ms) < 3 {
				ms += "0"
			}
			t := p[1]
			if len(t) == 7 { // „1:02:03“ → „01:02:03“
				t = "0" + t
			}
			return t + "." + ms
		})
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\n\n")
	}
	return []byte(b.String())
}
