/**
 * Tvar katalogu překladů. Zdrojem pravdy je angličtina (`locales/en`);
 * ostatní jazyky musí obsahovat všechny její klíče – chybějící klíč je chyba
 * typecheku. Navíc smí mít další plurálové tvary (`_few`, `_many`), které
 * angličtina nemá.
 *
 * Angličtina zůstává záložním jazykem pro i18next.
 */
export type Catalog<T> = {
  [K in keyof T]: T[K] extends string ? string : Catalog<T[K]>
} & { [plural: `${string}_${'zero' | 'two' | 'few' | 'many'}`]: string }

export type PartialCatalog<T> = {
  [K in keyof T]?: T[K] extends string ? string : PartialCatalog<T[K]>
} & { [plural: `${string}_${'zero' | 'two' | 'few' | 'many'}`]: string }
