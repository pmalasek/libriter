# core-web

React komponenty společné webovým rozhraním všech produktů: shadcn/ui
(`components/ui/*`), vzhled (téma, barevná schémata, paleta OKLCH
v `styles/base.css`), přihlášení a session (`auth/*`), jazyk rozhraní
(`i18n/*`), přihlašovací, registrační a profilová stránka a hooky účtu.

Jako `core-shared` se publikuje jako TypeScript zdroj bez build kroku.
Importuje se po souborech:

```ts
import { Button } from 'core-web/src/components/ui/button'
import { AuthProvider, useAuth } from 'core-web/src/auth/AuthContext'
```

Produkt dodá značku (logo, body na přihlašovací stránce) přes
`BrandProvider` z `core-web/brand` a klíč session přes
`configureSessionStorage` z `core-web/auth/session`. Styly:

```css
@import "core-web/src/styles/base.css";
```

`base.css` obsahuje `@source` na tento balíček, aby Tailwind našel třídy
použité ve sdílených komponentách. Novou shadcn komponentu dejte do
`src/components/ui/` tady; aplikace ji importují přímo z `core-web`.
