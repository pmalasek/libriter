# core-shared

Kód společný všem produktům (Libriter, Sceniter) a všem jejich aplikacím:
klient nad `fetch`, typy účtů a přihlášení, oprávnění, vzhled, formátování
a jedna instance i18next s texty jádra.

Balíček **nezávisí na Reactu ani na DOM** a publikuje se jako TypeScript
zdroj bez build kroku (Vite i Metro si `.ts` přeloží samy).

Aplikace ho neimportují přímo – importují balíček svého produktu
(`libriter-shared`, `sceniter-shared`), který jádro re-exportuje a přidá
vlastní typy a texty:

```ts
import { registerTranslations, type CoreCatalog } from 'core-shared'

registerTranslations({ en, cs, fr, de, es }) // hluboké sloučení s jádrem

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: CoreCatalog & typeof en }
  }
}
```

Katalogy jádra zatím obsahují i několik knižních popisků (`labels.bookStatus`,
`layout.nav.books` a podobně) z doby, kdy byl jen Libriter. Produkt je může
přepsat nebo nepoužít; jejich přesun do `libriter-shared` je samostatný krok.
