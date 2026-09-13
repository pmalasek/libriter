package main

import (
	"fmt"
	"log/slog"
	"os"
)

const usage = `Libriter - osobní správce a přehrávač audioknih

Použití:
  libriter [serve]              spustí HTTP server s webovým rozhraním (výchozí)
  libriter user <příkaz>        správa uživatelů z příkazové řádky
  libriter help                 vypíše tuto nápovědu

Příkazy user:
  user add       vytvoří nového uživatele
  user set-role  změní roli existujícího uživatele
  user list      vypíše všechny uživatele

Nápovědu k jednotlivým příkazům získáte přes: libriter user add -h
`

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	args := os.Args[1:]

	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}

	var err error
	switch cmd {
	case "serve":
		err = runServe()
	case "user":
		err = runUser(args[1:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "neznámý příkaz %q\n\n%s", cmd, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "chyba: %v\n", err)
		os.Exit(1)
	}
}
