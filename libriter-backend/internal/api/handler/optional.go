package handler

import "encoding/json"

// Optional rozlišuje "pole v JSON těle chybí" od "pole má hodnotu null".
// encoding/json volá UnmarshalJSON jen u klíčů, které v těle skutečně jsou,
// takže Set je true právě tehdy, když klient pole poslal.
//
// Používá se u částečných aktualizací (PATCH), kde nezaslané pole musí zůstat
// beze změny, ale explicitní null má vyprázdnit sloupec. U nullable polí
// se parametrizuje ukazatelem, např. Optional[*string].
type Optional[T any] struct {
	Set   bool
	Value T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &o.Value); err != nil {
		return err
	}
	o.Set = true
	return nil
}
