package service

import "errors"

// ErrInvalidSetting znamená, že poslané nastavení neprošlo kontrolou.
var ErrInvalidSetting = errors.New("neplatné nastavení")
