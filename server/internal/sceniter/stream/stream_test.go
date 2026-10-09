package stream

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
)

func s(v string) *string { return &v }

func media(container, video string, audio ...string) model.MediaFile {
	mf := model.MediaFile{FilePath: "Filmy/x." + container, Container: s(container), VideoCodec: s(video), PixFmt: s("yuv420p")}
	mf.Streams = append(mf.Streams, model.Stream{Index: 0, Kind: "video", Codec: s(video)})
	for i, a := range audio {
		mf.Streams = append(mf.Streams, model.Stream{Index: i + 1, Kind: "audio", Codec: s(a)})
	}
	return mf
}

func TestDecide(t *testing.T) {
	web, safari := ProfileFor("web"), ProfileFor("safari")
	one := 2
	cases := []struct {
		name string
		mf   model.MediaFile
		p    Profile
		aud  *int
		want string
	}{
		{"mp4 h264 aac", media("mp4", "h264", "aac"), web, nil, ModeDirect},
		{"mkv h264 aac (Docent)", media("matroska", "h264", "aac"), web, nil, ModeRemux},
		{"mkv h264 ac3 (CZ dabing) web", media("matroska", "h264", "ac3"), web, nil, ModeTranscodeAudio},
		{"mkv h264 ac3 safari", media("matroska", "h264", "ac3"), safari, nil, ModeRemux},
		{"avi mpeg4 mp3", media("avi", "mpeg4", "mp3"), web, nil, ModeTranscode},
		{"mp4 hevc web", media("mp4", "hevc", "aac"), web, nil, ModeTranscode},
		{"mp4 hevc safari", media("mp4", "hevc", "aac"), safari, nil, ModeDirect},
		{"mp4, druhá stopa", media("mp4", "h264", "aac", "aac"), web, &one, ModeRemux},
		{"neprozkoumané mp4", model.MediaFile{FilePath: "a.mp4"}, web, nil, ModeDirect},
		{"neprozkoumané avi", model.MediaFile{FilePath: "a.avi"}, web, nil, ModeTranscode},
	}
	for _, c := range cases {
		if got := Decide(c.mf, c.p, c.aud); got.Mode != c.want {
			t.Errorf("%s: %s (%v), chci %s", c.name, got.Mode, got.Reasons, c.want)
		}
	}

	tenBit := media("mp4", "h264", "aac")
	tenBit.PixFmt = s("yuv420p10le")
	if got := Decide(tenBit, web, nil); got.Mode != ModeTranscode {
		t.Errorf("10bit h264: %s", got.Mode)
	}
}

func TestDecideSubtitles(t *testing.T) {
	mf := media("matroska", "h264", "ac3")
	mf.Streams = append(mf.Streams,
		model.Stream{Index: 2, Kind: "subtitle", Codec: s("subrip"), Language: s("cs"), IsForced: true},
		model.Stream{Index: 3, Kind: "subtitle", Codec: s("hdmv_pgs_subtitle")})
	id := uuid.New()
	mf.Subtitles = []model.ExternalSubtitle{{ID: id, Format: "srt", Language: s("en")}}
	d := Decide(mf, ProfileFor("web"), nil)
	if len(d.Subtitles) != 2 || d.Subtitles[0].Key != "s2" || !d.Subtitles[0].Forced || d.Subtitles[1].Key != "x"+id.String() {
		t.Fatalf("titulky %+v", d.Subtitles)
	}
	if len(d.Unsupported) != 1 || d.Unsupported[0] != 3 {
		t.Fatalf("nepodporované %v", d.Unsupported)
	}
}

func TestRewritePlaylist(t *testing.T) {
	in := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:6.0,\nseg_00000.m4s\n#EXTINF:6.0,\r\nseg_00001.m4s\r\n"
	out := string(RewritePlaylist([]byte(in), "t=abc"))
	for _, want := range []string{`URI="init.mp4?t=abc"`, "seg_00000.m4s?t=abc\n", "seg_00001.m4s?t=abc\r\n", "#EXTINF:6.0,\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("chybí %q v:\n%s", want, out)
		}
	}
}

func TestResolve(t *testing.T) {
	if _, ok := Resolve("/data/video", "../etc/passwd"); ok {
		t.Error("únik z kořene")
	}
	if _, ok := Resolve("/data/video", "/etc/passwd"); ok {
		t.Error("absolutní cesta")
	}
	if p, ok := Resolve("/data/video", "Filmy/a b.mkv"); !ok || p != "/data/video/Filmy/a b.mkv" {
		t.Errorf("Resolve = %q %v", p, ok)
	}
}
