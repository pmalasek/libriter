package service

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"libriter/internal/config"
	"libriter/internal/storage"
	"libriter/internal/version"
)

// DiskUsage je volné a celkové místo svazku, na kterém leží audio soubory.
type DiskUsage struct {
	Path       string `json:"path"`
	FreeBytes  uint64 `json:"free_bytes"`
	TotalBytes uint64 `json:"total_bytes"`
}

// SystemInfo popisuje běžící server. Slouží k rychlé diagnostice: kam server
// zapisuje, jestli má ffprobe (bez něj se délka kapitol ukládá jako 1 s)
// a kolik zbývá místa.
type SystemInfo struct {
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	OS        string    `json:"os"`
	Env       string    `json:"env"`
	StartedAt time.Time `json:"started_at"`
	Uptime    int64     `json:"uptime_seconds"`

	DBPath          string `json:"db_path"`
	AudioRoot       string `json:"audio_root"`
	CoverRoot       string `json:"cover_root"`
	AuthorImageRoot string `json:"author_image_root"`

	FfprobeAvailable bool   `json:"ffprobe_available"`
	FfprobePath      string `json:"ffprobe_path"`

	// Disk je nil, pokud systém zjištění místa nepodporuje.
	Disk *DiskUsage `json:"disk"`
}

// SystemService skládá přehled knihovny a informace o běžícím serveru.
type SystemService struct {
	store     *storage.Store
	cfg       *config.Config
	startedAt time.Time
}

func NewSystem(store *storage.Store, cfg *config.Config, startedAt time.Time) *SystemService {
	return &SystemService{store: store, cfg: cfg, startedAt: startedAt}
}

func (s *SystemService) Info() SystemInfo {
	ffprobePath, ffprobeErr := exec.LookPath("ffprobe")

	info := SystemInfo{
		Version:          version.String(),
		GoVersion:        runtime.Version(),
		OS:               runtime.GOOS + "/" + runtime.GOARCH,
		Env:              s.cfg.Server.Env,
		StartedAt:        s.startedAt,
		Uptime:           int64(time.Since(s.startedAt).Seconds()),
		DBPath:           absPath(s.cfg.DB.Path),
		AudioRoot:        absPath(s.cfg.Storage.AudioRoot),
		CoverRoot:        absPath(s.cfg.Storage.CoverRoot),
		AuthorImageRoot:  absPath(s.cfg.Storage.AuthorImageRoot),
		FfprobeAvailable: ffprobeErr == nil,
		FfprobePath:      ffprobePath,
	}

	info.Disk = diskUsage(info.AudioRoot)
	return info
}

func (s *SystemService) Stats(ctx context.Context) (storage.LibraryStats, error) {
	return s.store.LibraryStats(ctx)
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
