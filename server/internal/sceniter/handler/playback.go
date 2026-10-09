package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
	"mediater/internal/sceniter/stream"
)

const apiBase = "/api/v1/sceniter"

// PlaybackResponse říká přehrávači, odkud a jak video hrát. Adresy už nesou
// streamovací token (?t=), klient je jen použije.
type PlaybackResponse struct {
	MediaID     uuid.UUID `json:"media_id"`
	URL         string    `json:"url"`
	HLS         bool      `json:"hls"`
	DurationSec *float64  `json:"duration_sec,omitempty"`
	TokenExpiry time.Time `json:"token_expires_at"`
	stream.Decision
	Subtitles []subtitleTrack `json:"subtitles"`
	// TranscodingAvailable je false, když server nemá ffmpeg – pak jde jen
	// přímé přehrávání a soubor nemusí v prohlížeči hrát.
	TranscodingAvailable bool `json:"transcoding_available"`
}

type subtitleTrack struct {
	stream.SubtitleOption
	URL string `json:"url"`
}

// GET /sceniter/media/{id}/playback?profile=web|safari&audio=<index>&mode=<vynutit režim>
func (h *Handler) Playback(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	mf, err := h.Store.GetMedia(r.Context(), id)
	if writeStoreError(w, err, "media.not_found") {
		return
	}
	d := stream.Decide(*mf, stream.ProfileFor(r.URL.Query().Get("profile")), intQuery(r, "audio"))

	// Klient může režim zvednout (přímé přehrání selhalo → remux), nikdy snížit
	// pod to, co soubor potřebuje.
	if forced := r.URL.Query().Get("mode"); forced != "" && modeRank(forced) > modeRank(d.Mode) {
		d.Mode = forced
		d.Reasons = append(d.Reasons, "režim vyžádal klient")
	}
	if d.Mode != stream.ModeDirect && !h.HLS.Available() {
		d.Reasons = append(d.Reasons, "server nemá ffmpeg, zkouší se přímé přehrání")
		d.Mode = stream.ModeDirect
	}

	token, expires, err := h.Auth.GenerateStreamToken(userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.internal", "token pro přehrávání se nepodařilo vydat")
		return
	}
	t := url.QueryEscape(token)

	resp := PlaybackResponse{MediaID: mf.ID, DurationSec: mf.DurationSec, TokenExpiry: expires, Decision: d,
		TranscodingAvailable: h.HLS.Available(), Subtitles: []subtitleTrack{}}
	if d.Mode == stream.ModeDirect {
		resp.URL = fmt.Sprintf("%s/media/%s/stream?t=%s", apiBase, mf.ID, t)
	} else {
		resp.HLS = true
		resp.URL = fmt.Sprintf("%s/media/%s/hls/master.m3u8?t=%s&mode=%s", apiBase, mf.ID, t, d.Mode)
		if d.AudioIndex != nil {
			resp.URL += "&audio=" + strconv.Itoa(*d.AudioIndex)
		}
	}
	for _, s := range d.Subtitles {
		resp.Subtitles = append(resp.Subtitles, subtitleTrack{SubtitleOption: s,
			URL: fmt.Sprintf("%s/media/%s/subtitles/%s.vtt?t=%s", apiBase, mf.ID, s.Key, t)})
	}
	writeJSON(w, http.StatusOK, resp)
}

func modeRank(m string) int {
	switch m {
	case stream.ModeDirect:
		return 0
	case stream.ModeRemux:
		return 1
	case stream.ModeTranscodeAudio:
		return 2
	case stream.ModeTranscode:
		return 3
	}
	return -1
}

// mediaFile načte soubor a jeho absolutní cestu; při chybě odpoví.
func (h *Handler) mediaFile(w http.ResponseWriter, r *http.Request) (*model.MediaFile, string, bool) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return nil, "", false
	}
	mf, err := h.Store.GetMedia(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media.not_found", "video nenalezeno")
		return nil, "", false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání videa")
		return nil, "", false
	}
	abs, ok := stream.Resolve(h.VideoRoot, mf.FilePath)
	if !ok {
		writeError(w, http.StatusNotFound, "media.not_found", "soubor videa nenalezen")
		return nil, "", false
	}
	return mf, abs, true
}

// GET|HEAD /sceniter/media/{id}/stream?t= – soubor tak, jak je (Range).
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.streamUser(w, r); !ok {
		return
	}
	_, abs, ok := h.mediaFile(w, r)
	if !ok {
		return
	}
	if !stream.ServeFile(w, r, abs) {
		writeError(w, http.StatusNotFound, "media.not_found", "soubor videa nenalezen")
	}
}

// GET /sceniter/media/{id}/hls/master.m3u8?t=&mode=&audio= – založí (nebo
// znovu použije) převod a vrátí master playlist.
func (h *Handler) Master(w http.ResponseWriter, r *http.Request) {
	user, ok := h.streamUser(w, r)
	if !ok {
		return
	}
	mf, abs, ok := h.mediaFile(w, r)
	if !ok {
		return
	}
	if _, err := os.Stat(abs); err != nil {
		writeError(w, http.StatusNotFound, "media.not_found", "soubor videa nenalezen")
		return
	}
	mode := r.URL.Query().Get("mode")
	if modeRank(mode) < 1 {
		writeError(w, http.StatusBadRequest, "validation.invalid", "neplatný režim převodu")
		return
	}
	opts := stream.Options{UserID: user, MediaID: mf.ID, Path: abs, Mode: mode, Audio: intQuery(r, "audio"),
		MaxHeight: h.MaxHeight, HEVC: mf.VideoCodec != nil && *mf.VideoCodec == "hevc"}
	if mf.DurationSec != nil {
		opts.Duration = *mf.DurationSec
	}
	if mf.Bitrate != nil {
		opts.Bitrate = *mf.Bitrate
	}
	if mf.Width != nil && mf.Height != nil {
		opts.Width, opts.Height = *mf.Width, *mf.Height
	}
	s, err := h.HLS.Open(opts)
	switch {
	case errors.Is(err, stream.ErrBusy):
		writeError(w, http.StatusServiceUnavailable, "transcode.busy", err.Error())
		return
	case errors.Is(err, stream.ErrNoFFmpeg):
		writeError(w, http.StatusServiceUnavailable, "transcode.unavailable", err.Error())
		return
	case err != nil:
		slog.Error("HLS převod", "media", mf.ID, "err", err)
		writeError(w, http.StatusInternalServerError, "transcode.failed", "převod videa se nepodařilo spustit")
		return
	}

	bandwidth := opts.Bitrate
	if mode == stream.ModeTranscode || bandwidth <= 0 {
		bandwidth = 8_000_000
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-INDEPENDENT-SEGMENTS\n")
	fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d", bandwidth)
	if opts.Width > 0 && opts.Height > 0 && mode != stream.ModeTranscode {
		fmt.Fprintf(&b, ",RESOLUTION=%dx%d", opts.Width, opts.Height)
	}
	fmt.Fprintf(&b, "\n%s/hls/%s/index.m3u8?t=%s\n", apiBase, s.ID, url.QueryEscape(r.URL.Query().Get("t")))

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Sceniter-Session", s.ID)
	_, _ = w.Write([]byte(b.String()))
}

// GET /sceniter/hls/{session}/{file}?t= – playlist, init segment a segmenty.
func (h *Handler) HLSFile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.streamUser(w, r)
	if !ok {
		return
	}
	s, err := h.HLS.Lookup(chi.URLParam(r, "session"), user)
	if err != nil {
		writeError(w, http.StatusNotFound, "transcode.gone", "převod skončil, spusťte přehrávání znovu")
		return
	}
	name := chi.URLParam(r, "file")
	stream.NoWriteDeadline(w)

	if name == "index.m3u8" {
		data, err := h.HLS.Playlist(r.Context(), s)
		if err != nil {
			h.hlsError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(stream.RewritePlaylist(data, "t="+url.QueryEscape(r.URL.Query().Get("t"))))
		return
	}

	path, err := h.HLS.Segment(r.Context(), s, name)
	if err != nil {
		h.hlsError(w, err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, path)
}

func (h *Handler) hlsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, stream.ErrSegmentTimeout):
		w.Header().Set("Retry-After", "2")
		writeError(w, http.StatusServiceUnavailable, "transcode.pending", err.Error())
	case errors.Is(err, stream.ErrSessionGone):
		writeError(w, http.StatusNotFound, "transcode.gone", "převod skončil, spusťte přehrávání znovu")
	default:
		slog.Warn("HLS", "err", err)
		writeError(w, http.StatusInternalServerError, "transcode.failed", "převod videa selhal")
	}
}

// DELETE /sceniter/hls/{session}?t= (i POST …/stop) – přehrávač odchází.
func (h *Handler) HLSStop(w http.ResponseWriter, r *http.Request) {
	user, ok := h.streamUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "session")
	if _, err := h.HLS.Lookup(id, user); err == nil {
		h.HLS.Stop(id)
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /sceniter/media/{id}/subtitles/{key}.vtt?t= – titulky ve WebVTT.
// key je „s<index>“ (vložená stopa) nebo „x<uuid>“ (soubor vedle videa).
func (h *Handler) Subtitle(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.streamUser(w, r); !ok {
		return
	}
	mf, abs, ok := h.mediaFile(w, r)
	if !ok {
		return
	}
	key := strings.TrimSuffix(chi.URLParam(r, "key"), ".vtt")
	cacheKey := mf.ID.String() + "-" + key
	if mf.ModTime != nil {
		cacheKey += "-" + strconv.FormatInt(mf.ModTime.Unix(), 10)
	}

	var (
		data []byte
		err  error
	)
	switch {
	case strings.HasPrefix(key, "s"):
		idx, perr := strconv.Atoi(key[1:])
		found := false
		for _, st := range mf.Streams {
			found = found || (st.Kind == "subtitle" && st.Index == idx)
		}
		if perr != nil || !found {
			writeError(w, http.StatusNotFound, "subtitle.not_found", "titulky nenalezeny")
			return
		}
		data, err = h.Subs.Embedded(r.Context(), cacheKey, abs, idx)
	case strings.HasPrefix(key, "x"):
		var sub *model.ExternalSubtitle
		for i := range mf.Subtitles {
			if "x"+mf.Subtitles[i].ID.String() == key {
				sub = &mf.Subtitles[i]
			}
		}
		if sub == nil {
			writeError(w, http.StatusNotFound, "subtitle.not_found", "titulky nenalezeny")
			return
		}
		subAbs, ok := stream.Resolve(h.VideoRoot, sub.FilePath)
		if !ok {
			writeError(w, http.StatusNotFound, "subtitle.not_found", "titulky nenalezeny")
			return
		}
		data, err = h.Subs.External(r.Context(), cacheKey, subAbs, sub.Format)
	default:
		writeError(w, http.StatusNotFound, "subtitle.not_found", "titulky nenalezeny")
		return
	}
	if err != nil {
		slog.Warn("titulky", "media", mf.ID, "key", key, "err", err)
		writeError(w, http.StatusUnprocessableEntity, "subtitle.unsupported", "titulky nejde převést")
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(data)
}
