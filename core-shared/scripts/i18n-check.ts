// Přehled úplnosti překladů jádra proti angličtině. Chybějící klíč je chyba
// (i v typechecku).
//
//   npm run i18n:check -w core-shared            souhrn
//   npm run i18n:check -w core-shared -- --list  i s klíči navíc

import { checkCatalogs, coreCatalogs } from '../src'

if (!checkCatalogs('jádro', coreCatalogs, { list: process.argv.includes('--list') })) process.exitCode = 1
