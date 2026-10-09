package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
)

// probeTimeout je strop na jeden běh ffprobe (soubor na pomalém síťovém disku).
const probeTimeout = 2 * time.Minute

type ffprobeOutput struct {
	Streams []struct {
		Index       int               `json:"index"`
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Profile     string            `json:"profile"`
		Width       int               `json:"width"`
		Height      int               `json:"height"`
		PixFmt      string            `json:"pix_fmt"`
		Channels    int               `json:"channels"`
		Tags        map[string]string `json:"tags"`
		Disposition map[string]int    `json:"disposition"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
}

// Probe zjistí kontejner, délku a stopy souboru přes ffprobe. Selhání se
// vrací jako Probe.Error – soubor se přesto založí (třeba jde přehrát).
func Probe(ctx context.Context, ffprobe, path string) *storage.Probe {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return &storage.Probe{Error: truncate(msg, 500)}
	}
	return parseProbe(stdout.Bytes())
}

func parseProbe(data []byte) *storage.Probe {
	var out ffprobeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return &storage.Probe{Error: fmt.Sprintf("výstup ffprobe: %v", err)}
	}
	p := &storage.Probe{Container: containerName(out.Format.FormatName)}
	p.DurationSec, _ = strconv.ParseFloat(out.Format.Duration, 64)
	p.Bitrate, _ = strconv.ParseInt(out.Format.BitRate, 10, 64)

	for _, s := range out.Streams {
		kind := s.CodecType
		if kind != "video" && kind != "audio" && kind != "subtitle" {
			continue
		}
		// Obrázek přiložený k souboru (plakát v MKV) není stopa videa.
		if kind == "video" && s.Disposition["attached_pic"] == 1 {
			continue
		}
		st := model.Stream{
			Index:     s.Index,
			Kind:      kind,
			Codec:     strPtr(s.CodecName),
			Title:     strPtr(s.Tags["title"]),
			IsDefault: s.Disposition["default"] == 1,
			IsForced:  s.Disposition["forced"] == 1 || strings.EqualFold(strings.TrimSpace(s.Tags["title"]), "forced"),
		}
		if lang := NormalizeLanguage(s.Tags["language"]); lang != "" {
			st.Language = &lang
		} else if raw := strings.TrimSpace(s.Tags["language"]); raw != "" && raw != "und" {
			st.Language = &raw
		}
		if kind == "audio" && s.Channels > 0 {
			ch := s.Channels
			st.Channels = &ch
		}
		if kind == "video" && p.VideoCodec == "" {
			p.VideoCodec, p.VideoProfile = s.CodecName, s.Profile
			p.Width, p.Height, p.PixFmt = s.Width, s.Height, s.PixFmt
		}
		p.Streams = append(p.Streams, st)
	}
	return p
}

// containerName převede format_name z ffprobe ("matroska,webm",
// "mov,mp4,m4a,3gp,3g2,mj2") na jedno jméno, se kterým počítá rozhodování
// o přehrávání.
func containerName(format string) string {
	switch {
	case strings.HasPrefix(format, "matroska"):
		return "matroska"
	case strings.HasPrefix(format, "mov,mp4"):
		return "mp4"
	case format == "":
		return ""
	}
	name, _, _ := strings.Cut(format, ",")
	return name
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
