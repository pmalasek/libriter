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

// keyframeWindows: jak daleko před cílem se hledá klíčový snímek (s). Běžné
// GOP mají do 10 s; delší okno se zkusí, jen když kratší nic nenajde.
var keyframeWindows = []float64{15, 60}

// keyframeBefore najde čas posledního klíčového snímku obrazu nejvýš v at.
// Přebalení (-c:v copy) umí začít jen na klíčovém snímku – s jeho přesným
// časem v -ss ffmpeg začne právě tam a přehrávač zná posun časové osy.
func keyframeBefore(ctx context.Context, ffprobe, path string, at float64) (float64, error) {
	if at <= 0 {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for _, win := range keyframeWindows {
		from := max(0, at-win)
		// Konec intervalu ffprobe nečte – snímek přesně v at by vypadl.
		out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0",
			"-read_intervals", fmt.Sprintf("%.3f%%%.3f", from, at+0.01),
			"-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path).Output()
		if err != nil {
			return 0, fmt.Errorf("ffprobe: %w", err)
		}
		if k, ok := lastKeyframe(out, at); ok {
			return k, nil
		}
		if from == 0 {
			break
		}
	}
	return 0, nil
}

// lastKeyframe vybere z výstupu ffprobe (řádky "pts_time,flags") nejpozdější
// klíčový snímek nejvýš v at.
func lastKeyframe(out []byte, at float64) (float64, bool) {
	best, found := 0.0, false
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		pts, flags, ok := strings.Cut(strings.TrimSpace(sc.Text()), ",")
		if !ok || !strings.HasPrefix(flags, "K") {
			continue
		}
		t, err := strconv.ParseFloat(pts, 64)
		if err != nil || t > at {
			continue
		}
		if !found || t > best {
			best, found = t, true
		}
	}
	return best, found
}
