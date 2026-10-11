// Package stream rozhoduje, jak video doručit klientovi, a doručuje ho:
// přímo (soubor s Range), přes HLS s přebalením (remux), s převodem zvuku,
// nebo s plným převodem obrazu i zvuku. Titulky převádí do WebVTT.
package stream

import (
	"slices"
	"strconv"
	"strings"

	"mediater/internal/sceniter/model"
)

// Režimy přehrávání, od nejlevnějšího.
const (
	ModeDirect         = "direct"          // soubor tak, jak je, s Range
	ModeRemux          = "remux"           // HLS, obraz i zvuk beze změny
	ModeTranscodeAudio = "transcode_audio" // HLS, obraz beze změny, zvuk do AAC
	ModeTranscode      = "transcode"       // HLS, obraz do H.264, zvuk do AAC
)

// Profile popisuje, co klient umí přehrát bez pomoci serveru.
type Profile struct {
	Name       string
	Containers []string
	Video      []string
	Audio      []string
	// MaxHeight > 0 omezuje výšku obrazu pro přímé přehrání.
	MaxHeight int
}

// Profiles jsou známé profily klientů: prohlížeče a televizní aplikace.
var Profiles = map[string]Profile{
	// Chrome, Firefox, Edge: <video> s MSE (hls.js). MKV ani AC3 neumí spolehlivě.
	"web": {
		Name:       "web",
		Containers: []string{"mp4", "webm"},
		Video:      []string{"h264", "vp8", "vp9", "av1"},
		Audio:      []string{"aac", "mp3", "opus", "vorbis", "flac"},
	},
	// Safari na macOS a iOS: nativní HLS, HEVC a AC3/E-AC3 umí.
	"safari": {
		Name:       "safari",
		Containers: []string{"mp4"},
		Video:      []string{"h264", "hevc"},
		Audio:      []string{"aac", "mp3", "ac3", "eac3", "flac", "alac"},
	},
	// Apple TV (AVPlayer): jako Safari – MP4/MOV a HLS, HEVC, Dolby zvuk.
	// MKV jde přes remux, obraz se nepřevádí.
	"tvos": {
		Name:       "tvos",
		Containers: []string{"mp4"},
		Video:      []string{"h264", "hevc"},
		Audio:      []string{"aac", "mp3", "ac3", "eac3", "flac", "alac"},
	},
	// Google TV / Android TV (ExoPlayer): MKV přehraje přímo. AC3/E-AC3 umí
	// jen některé přístroje (passthrough do TV či receiveru) a AV1 jen novější
	// hardware, proto chybí – převod samotného zvuku je levný.
	"androidtv": {
		Name:       "androidtv",
		Containers: []string{"mp4", "matroska", "webm"},
		Video:      []string{"h264", "hevc", "vp9"},
		Audio:      []string{"aac", "mp3", "opus", "vorbis", "flac"},
	},
}

// ProfileFor vrátí profil podle jména; neznámé jméno dostane „web“.
func ProfileFor(name string) Profile {
	if p, ok := Profiles[strings.ToLower(name)]; ok {
		return p
	}
	return Profiles["web"]
}

// textSubtitleCodecs jsou vložené titulky, které jde převést do WebVTT.
// Obrázkové (hdmv_pgs_subtitle, dvd_subtitle) by se musely vypálit do obrazu.
var textSubtitleCodecs = []string{"subrip", "srt", "ass", "ssa", "mov_text", "webvtt", "text"}

// SubtitleOption je titulková stopa, kterou může klient zobrazit.
type SubtitleOption struct {
	// Key je „s<index streamu>“ u vložených, „x<uuid>“ u externích titulků.
	Key      string  `json:"key"`
	Language *string `json:"language,omitempty"`
	Title    *string `json:"title,omitempty"`
	Forced   bool    `json:"forced"`
	External bool    `json:"external"`
	Default  bool    `json:"default"`
}

// Decision je výsledek rozhodnutí o přehrávání.
type Decision struct {
	Mode       string           `json:"mode"`
	Reasons    []string         `json:"reasons"`
	AudioIndex *int             `json:"audio_index,omitempty"`
	Audio      []model.Stream   `json:"audio"`
	Subtitles  []SubtitleOption `json:"subtitles"`
	// Unsupported nese vložené titulky, které nejdou zobrazit (obrázkové).
	Unsupported []int `json:"unsupported_subtitles"`
}

// Decide vybere nejlevnější režim, ve kterém klient s profilem p soubor
// přehraje. audio je požadovaný index zvukové stopy (nil = výchozí).
func Decide(mf model.MediaFile, p Profile, audio *int) Decision {
	d := Decision{Audio: []model.Stream{}, Subtitles: []SubtitleOption{}, Unsupported: []int{}, Reasons: []string{}}

	var audios []model.Stream
	for _, s := range mf.Streams {
		switch s.Kind {
		case "audio":
			audios = append(audios, s)
		case "subtitle":
			codec := deref(s.Codec)
			if slices.Contains(textSubtitleCodecs, codec) {
				d.Subtitles = append(d.Subtitles, SubtitleOption{Key: "s" + strconv.Itoa(s.Index),
					Language: s.Language, Title: s.Title, Forced: s.IsForced, Default: s.IsDefault})
			} else {
				d.Unsupported = append(d.Unsupported, s.Index)
			}
		}
	}
	for _, x := range mf.Subtitles {
		d.Subtitles = append(d.Subtitles, SubtitleOption{Key: "x" + x.ID.String(),
			Language: x.Language, Forced: x.IsForced, External: true})
	}
	d.Audio = append(d.Audio, audios...)

	chosen := chooseAudio(audios, audio)
	if chosen != nil {
		idx := chosen.Index
		d.AudioIndex = &idx
	}

	// Bez údajů z ffprobe rozhoduje jen kontejner podle přípony.
	if mf.VideoCodec == nil {
		c := containerFromPath(mf.FilePath)
		if slices.Contains(p.Containers, c) {
			d.Mode = ModeDirect
			d.Reasons = append(d.Reasons, "soubor nebyl prozkoumán, kontejner "+c+" klient zná")
		} else {
			d.Mode = ModeTranscode
			d.Reasons = append(d.Reasons, "soubor nebyl prozkoumán")
		}
		return d
	}

	container := deref(mf.Container)
	containerOK := slices.Contains(p.Containers, container)
	videoOK := slices.Contains(p.Video, *mf.VideoCodec) && !tenBitH264(mf) &&
		(p.MaxHeight == 0 || mf.Height == nil || *mf.Height <= p.MaxHeight)
	audioOK := chosen == nil || slices.Contains(p.Audio, deref(chosen.Codec))
	// Prohlížeč přehraje jen první zvukovou stopu souboru; jiná vybraná
	// stopa vyžaduje přebalení.
	firstAudio := chosen == nil || len(audios) == 0 || chosen.Index == audios[0].Index

	if !containerOK {
		d.Reasons = append(d.Reasons, "kontejner "+container+" klient nepřehraje")
	}
	if !videoOK {
		d.Reasons = append(d.Reasons, "obraz "+*mf.VideoCodec+" klient nepřehraje")
	}
	if !audioOK {
		d.Reasons = append(d.Reasons, "zvuk "+deref(chosen.Codec)+" klient nepřehraje")
	}
	if !firstAudio {
		d.Reasons = append(d.Reasons, "vybraná zvuková stopa není první")
	}

	switch {
	case containerOK && videoOK && audioOK && firstAudio:
		d.Mode = ModeDirect
	case videoOK && audioOK:
		d.Mode = ModeRemux
	case videoOK:
		d.Mode = ModeTranscodeAudio
	default:
		d.Mode = ModeTranscode
	}
	return d
}

// chooseAudio vybere zvukovou stopu: požadovanou, jinak výchozí, jinak první.
func chooseAudio(audios []model.Stream, want *int) *model.Stream {
	if len(audios) == 0 {
		return nil
	}
	if want != nil {
		for i := range audios {
			if audios[i].Index == *want {
				return &audios[i]
			}
		}
	}
	for i := range audios {
		if audios[i].IsDefault {
			return &audios[i]
		}
	}
	return &audios[0]
}

// tenBitH264 – 10bitové H.264 (Hi10P) prohlížeče nedekódují.
func tenBitH264(mf model.MediaFile) bool {
	return deref(mf.VideoCodec) == "h264" && strings.Contains(deref(mf.PixFmt), "10")
}

func containerFromPath(p string) string {
	switch strings.ToLower(p[strings.LastIndex(p, ".")+1:]) {
	case "mp4", "m4v", "mov":
		return "mp4"
	case "webm":
		return "webm"
	case "mkv":
		return "matroska"
	}
	return ""
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
