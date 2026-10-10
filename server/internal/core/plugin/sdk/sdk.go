// Package sdk usnadňuje psaní pluginů metadat v Go: přečte příkaz
// a požadavek, zavolá obsluhu a zapíše odpověď podle protokolu
// (viz balíček plugin a docs/plugins.md).
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"mediater/internal/core/plugin"
)

// Plugin je obsluha příkazů. Nevyplněný příkaz odpoví bad_request.
type Plugin struct {
	Manifest plugin.Manifest
	Search   func(ctx context.Context, req plugin.Request) ([]plugin.SearchResult, error)
	// Fetch vrací položku, která se zapíše do {"item": …}.
	Fetch  func(ctx context.Context, req plugin.Request) (any, error)
	Season func(ctx context.Context, req plugin.Request) (*plugin.SeasonMetadata, error)
}

// Errorf vrátí chybu s kódem protokolu.
func Errorf(code, format string, args ...any) error {
	return &plugin.Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// NotFound vrátí chybu not_found.
func NotFound(format string, args ...any) error {
	return Errorf(plugin.ErrCodeNotFound, format, args...)
}

// BadRequest vrátí chybu bad_request.
func BadRequest(format string, args ...any) error {
	return Errorf(plugin.ErrCodeBadRequest, format, args...)
}

// Run obslouží jeden příkaz a ukončí proces.
func Run(p Plugin) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	code := Serve(ctx, p, os.Args[1:], os.Stdin, os.Stdout)
	stop()
	os.Exit(code)
}

// Serve obslouží příkaz args[0] s požadavkem ze stdin a vrátí exit kód.
func Serve(ctx context.Context, p Plugin, args []string, stdin io.Reader, stdout io.Writer) int {
	if len(args) == 0 {
		return fail(stdout, BadRequest("chybí příkaz (describe, search, fetch, season)"))
	}
	cmd := args[0]
	if cmd == plugin.CmdDescribe {
		m := p.Manifest
		m.Protocol = plugin.ProtocolVersion
		return ok(stdout, m)
	}

	var req plugin.Request
	if err := json.NewDecoder(io.LimitReader(stdin, 1<<20)).Decode(&req); err != nil {
		return fail(stdout, BadRequest("neplatný požadavek: %v", err))
	}
	if req.Kind != "" && !p.Manifest.Supports(req.Kind) {
		return fail(stdout, BadRequest("druh %q není podporovaný", req.Kind))
	}

	switch {
	case cmd == plugin.CmdSearch && p.Search != nil:
		if req.Query == nil {
			return fail(stdout, BadRequest("chybí query"))
		}
		res, err := p.Search(ctx, req)
		if err != nil {
			return fail(stdout, err)
		}
		if res == nil {
			res = []plugin.SearchResult{}
		}
		return ok(stdout, plugin.SearchResponse{Results: res})
	case cmd == plugin.CmdFetch && p.Fetch != nil:
		if req.ID == "" && req.URL == "" {
			return fail(stdout, BadRequest("chybí id nebo url"))
		}
		item, err := p.Fetch(ctx, req)
		if err != nil {
			return fail(stdout, err)
		}
		return ok(stdout, map[string]any{"item": item})
	case cmd == plugin.CmdSeason && p.Season != nil:
		s, err := p.Season(ctx, req)
		if err != nil {
			return fail(stdout, err)
		}
		return ok(stdout, plugin.SeasonResponse{Season: *s})
	}
	return fail(stdout, BadRequest("neznámý příkaz %q", cmd))
}

func ok(w io.Writer, v any) int {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return 1
	}
	return 0
}

func fail(w io.Writer, err error) int {
	body := plugin.ErrorBody{Code: plugin.ErrCodeUnavailable, Message: err.Error()}
	var pe *plugin.Error
	if errors.As(err, &pe) {
		body.Code, body.Message = pe.Code, pe.Message
	}
	slog.Error("plugin", "code", body.Code, "err", body.Message)
	_ = json.NewEncoder(w).Encode(plugin.ErrorResponse{Error: &body})
	return 1
}
