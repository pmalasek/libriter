# Inkrementální synchronizace knihovny (`GET /api/v1/sync/changes`)

API pro budoucí verzi mobilní aplikace. Místo stahování celých seznamů
(`GET /books`, `/authors`, `/series`, `/books/progress`) si zařízení drží
lokální kopii a stahuje jen změny od poslední synchronizace. Stávající
endpointy beze změny dál fungují pro vydané verze aplikace.

## Princip

Každá změna knihy, autora, série a stavu poslechu (`book_progress`) dostane
pořadové číslo **rev** z jediného čítače (`sync_rev`). Smazání se zapíše jako
tombstone do `deleted_items` se stejnou řadou rev. Všechno udržují triggery
v databázi (migrace `libriter/001`), takže platí pro každou cestu zápisu: API,
scanner, slučování duplicit i opravy knihovny.

## Dotaz

```
GET /api/v1/sync/changes?since=<rev>&limit=<n>
Authorization: Bearer <token>
```

- `since`: rev z minulé odpovědi; bez něj (nebo `0`) jde o celou knihovnu.
- `limit`: počet změn v dávce (výchozí 500, nejvýš 2000).

## Odpověď

```json
{
  "rev": 1234,
  "more": false,
  "full_resync": false,
  "books": [ /* jako GET /books, včetně autorů */ ],
  "authors": [ /* jako GET /authors */ ],
  "series": [ /* jako GET /series */ ],
  "progress": [ /* jako GET /books/progress, jen přihlášený uživatel */ ],
  "deleted": [ { "kind": "book" | "author" | "series" | "progress", "id": "…" } ]
}
```

- Změny v dávce jsou seřazené podle rev. Záznamy se ukládají jako upsert
  podle `id`. U `progress` je `id` v `deleted` ID knihy.
- `more: true` znamená, že za `rev` čekají další změny. Klient se má hned
  zeptat znovu se `since = rev`.
- `full_resync: true` znamená, že dávka začíná od nuly. Klient zahodí svou
  kopii a z této a dalších dávek (`more`) složí celou knihovnu. Přijde
  při prvním dotazu, když server mezitím uklidil tombstony starší než
  `since` (drží se 90 dní), a když je `since` vyšší než stav serveru,
  například po obnově databáze ze zálohy.

## Doporučený postup klienta

1. Načíst uložený `rev` (zatím žádný → 0).
2. Volat `/sync/changes?since=rev`, dokud je `more` true. Každou dávku uložit
   v jedné lokální transakci i s novým `rev`.
3. Při `full_resync` začít s prázdnou kopií (smazat lokální data v téže
   transakci jako zápis první dávky).

Přejmenování autora pošle autora i všechny jeho knihy, protože knihy nesou
autory vnořené jako v `GET /books`.
