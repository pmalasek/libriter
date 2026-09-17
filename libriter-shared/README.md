# libriter-shared

Kód společný webovému rozhraní a mobilní aplikaci: typy API, klient nad
`fetch`, čisté pomocné funkce přehrávače a popisky poslechu.

Balíček **nezávisí na Reactu ani na DOM**. Všechno, co se pojí s konkrétní
platformou – ukládání session (`localStorage` versus `SecureStore`), react-query
hooky, komponenty – zůstává v příslušné aplikaci. Sdílí se jen to, co by se
jinak psalo dvakrát a rozešlo se.

Publikuje se TypeScript zdroj bez build kroku: Vite i Metro si `.ts` přeloží
samy, a odpadá tím krok, na který by se dalo zapomenout.

Jediná věc, kterou musí aplikace nastavit, je adresa serveru:

```ts
import { setBaseUrl } from 'libriter-shared'

setBaseUrl('https://libriter.example.com') // web nechává prázdné (stejný původ)
```
