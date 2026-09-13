package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"
)

const userUsage = `Správa uživatelů z příkazové řádky.

Použití:
  libriter user add --email <email> --name <jméno> [--role <role>] [--password <heslo>]
  libriter user set-role --email <email> --role <role>
  libriter user list

Role: admin | editor | reader (výchozí reader)
Bez --password se heslo zadává interaktivně (skrytě, dvakrát pro kontrolu).
`

const minPasswordLen = 8

// runUser obsluhuje podpříkazy `libriter user ...`.
func runUser(args []string) error {
	if len(args) == 0 {
		fmt.Print(userUsage)
		return errors.New("chybí podpříkaz")
	}

	switch args[0] {
	case "add":
		return runUserAdd(args[1:])
	case "set-role":
		return runUserSetRole(args[1:])
	case "list":
		return runUserList(args[1:])
	case "help", "-h", "--help":
		fmt.Print(userUsage)
		return nil
	default:
		fmt.Print(userUsage)
		return fmt.Errorf("neznámý podpříkaz %q", args[0])
	}
}

func runUserAdd(args []string) error {
	fs := flag.NewFlagSet("user add", flag.ContinueOnError)
	email := fs.String("email", "", "email uživatele (povinné)")
	name := fs.String("name", "", "zobrazované jméno (povinné)")
	role := fs.String("role", model.RoleReader, "role: admin | editor | reader")
	password := fs.String("password", "", "heslo; bez tohoto přepínače se zadá interaktivně")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Použití: libriter user add --email <email> --name <jméno> [--role <role>] [--password <heslo>]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	*email = strings.TrimSpace(*email)
	*name = strings.TrimSpace(*name)
	*role = strings.ToLower(strings.TrimSpace(*role))

	if *email == "" || *name == "" {
		fs.Usage()
		return errors.New("--email a --name jsou povinné")
	}
	if _, ok := model.RoleLevel[*role]; !ok {
		return fmt.Errorf("neznámá role %q (povolené: admin, editor, reader)", *role)
	}

	pass := *password
	if pass == "" {
		var err error
		if pass, err = promptPassword(); err != nil {
			return err
		}
	}
	if len(pass) < minPasswordLen {
		return fmt.Errorf("heslo musí mít alespoň %d znaků", minPasswordLen)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userSvc, closeDB, err := openUserService(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	u, err := userSvc.Create(ctx, *name, *email, pass, *role)
	if errors.Is(err, service.ErrEmailTaken) {
		return fmt.Errorf("uživatel s emailem %s už existuje", *email)
	}
	if err != nil {
		return err
	}

	fmt.Printf("Uživatel vytvořen:\n  id:    %s\n  jméno: %s\n  email: %s\n  role:  %s\n",
		u.ID, u.DisplayName, u.Email, u.Role)
	return nil
}

func runUserSetRole(args []string) error {
	fs := flag.NewFlagSet("user set-role", flag.ContinueOnError)
	email := fs.String("email", "", "email uživatele (povinné)")
	role := fs.String("role", "", "nová role: admin | editor | reader (povinné)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Použití: libriter user set-role --email <email> --role <role>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	*email = strings.TrimSpace(*email)
	*role = strings.ToLower(strings.TrimSpace(*role))

	if *email == "" || *role == "" {
		fs.Usage()
		return errors.New("--email a --role jsou povinné")
	}
	if _, ok := model.RoleLevel[*role]; !ok {
		return fmt.Errorf("neznámá role %q (povolené: admin, editor, reader)", *role)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userSvc, closeDB, err := openUserService(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	u, err := userSvc.GetByEmail(ctx, *email)
	if errors.Is(err, service.ErrNotFound) {
		return fmt.Errorf("uživatel s emailem %s nenalezen", *email)
	}
	if err != nil {
		return err
	}

	if err := userSvc.SetRole(ctx, u.ID, *role); err != nil {
		return err
	}

	fmt.Printf("Role uživatele %s změněna na %s.\n", u.Email, *role)
	return nil
}

func runUserList(args []string) error {
	fs := flag.NewFlagSet("user list", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprintln(fs.Output(), "Použití: libriter user list") }
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userSvc, closeDB, err := openUserService(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	users, err := userSvc.List(ctx)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		fmt.Println("Žádní uživatelé. Prvního vytvoříte přes: libriter user add --email … --name … --role admin")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ROLE\tEMAIL\tJMÉNO\tID")
	for _, u := range users {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.Role, u.Email, u.DisplayName, u.ID)
	}
	return w.Flush()
}

// openUserService otevře databázi (včetně migrací) a vrátí UserService
// spolu s funkcí pro uzavření spojení. Nespouští scanner ani HTTP server.
func openUserService(ctx context.Context) (*service.UserService, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("konfigurace: %w", err)
	}

	sqlDB, err := db.Open(ctx, cfg.DB)
	if err != nil {
		return nil, nil, fmt.Errorf("databáze: %w", err)
	}

	return service.NewUser(storage.New(sqlDB)), func() { _ = sqlDB.Close() }, nil
}

// promptPassword načte heslo dvakrát ze terminálu bez zobrazení znaků.
func promptPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("stdin není terminál – zadejte heslo přes --password")
	}

	fmt.Print("Heslo: ")
	first, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("čtení hesla: %w", err)
	}

	fmt.Print("Heslo znovu: ")
	second, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("čtení hesla: %w", err)
	}

	if string(first) != string(second) {
		return "", errors.New("hesla se neshodují")
	}
	return string(first), nil
}
