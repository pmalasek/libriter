package service

import (
	"fmt"
	"regexp"
	"strings"
)

// loginPattern povoluje písmena, číslice, tečku, podtržítko a pomlčku.
// Zavináč chybí záměrně – přihlášení porovnává zadaný text s loginem i
// e-mailem, a login se zavináčem by se mohl shodovat s cizím e-mailem.
var loginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

// NormalizeLogin ořízne mezery a ověří formát přihlašovacího jména. Prázdný
// login je platný – znamená, že uživatel login nemá.
func NormalizeLogin(login string) (string, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return "", nil
	}
	if !loginPattern.MatchString(login) {
		return "", fmt.Errorf("%w: 3–32 znaků, jen písmena bez diakritiky, číslice, tečka, podtržítko a pomlčka", ErrInvalidLogin)
	}
	return login, nil
}
