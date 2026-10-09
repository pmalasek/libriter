# sceniter-shared

Kód Sceniteru (filmy a seriály) bez Reactu a bez DOM: typy API
(`/api/v1/sceniter/...`), klíče react-query, formátovací pomocníci
(kód dílu, rozlišení, popisky stop, rozkoukanost) a texty v pěti jazycích.

Re-exportuje `core-shared`, takže aplikace importují jen tento balíček.
Při importu přidá své texty do sdílené instance i18next a přepíše několik
textů jádra (název aplikace, úvod přihlašovací stránky).

Chybové kódy serveru, které vrací jen Sceniter (`sceniter.*`, `transcode.*`,
`media.*` …), jsou v `src/i18n/locales/*/errors.ts` a slučují se do `errors`
jádra.
