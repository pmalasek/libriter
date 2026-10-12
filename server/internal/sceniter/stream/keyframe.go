package stream

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// seekLanding zjistí, kde převod spuštěný s -ss at (s od začátku souboru,
// jak ho chápe ffmpeg) skutečně začne. Kopírovaný obraz začíná na klíčovém
// snímku, na který demuxer přistane – a to bývá i o několik sekund dřív
// (MKV z ffmpegu má index jen po clusterech). Proto se přistání zjistí
// stejným ffmpeg a stejným -ss a vrátí se jako čas od začátku souboru:
// nula playlistu nového převodu v časové ose přehrávače.
func seekLanding(ctx context.Context, ffmpeg, ffprobe, path string, at float64) (float64, error) {
	if at <= 0 {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// -ss se počítá od začátku souboru, -copyts ale vypíše absolutní čas –
	// začátek souboru (u nahrávek z vysílání i několik sekund) se odečte.
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "format=start_time",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	fileStart, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)

	out, err = exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error",
		"-ss", strconv.FormatFloat(at, 'f', 3, 64), "-noaccurate_seek", "-i", path,
		"-map", "0:v:0", "-c", "copy", "-copyts", "-frames:v", "1", "-f", "framecrc", "-").Output()
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: %w", err)
	}
	pts, ok := firstPacketPTS(out)
	if !ok {
		return 0, fmt.Errorf("ffmpeg nevypsal první snímek")
	}
	return max(0, pts-fileStart), nil
}

// firstPacketPTS přečte pts prvního paketu z výstupu framecrc
// ("#tb 0: 1/1000" a řádky "stream, dts, pts, duration, size, hash").
func firstPacketPTS(out []byte) (float64, bool) {
	num, den := 0.0, 0.0
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if tb, ok := strings.CutPrefix(line, "#tb 0:"); ok {
			n, d, _ := strings.Cut(strings.TrimSpace(tb), "/")
			num, _ = strconv.ParseFloat(n, 64)
			den, _ = strconv.ParseFloat(d, 64)
			continue
		}
		if strings.HasPrefix(line, "#") || den == 0 {
			continue
		}
		f := strings.Split(line, ",")
		if len(f) < 3 {
			continue
		}
		pts, err := strconv.ParseFloat(strings.TrimSpace(f[2]), 64)
		if err != nil {
			return 0, false
		}
		return pts * num / den, true
	}
	return 0, false
}
