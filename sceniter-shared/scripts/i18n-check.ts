// Přehled úplnosti překladů Sceniteru (a jádra) proti angličtině.
//
//   npm run i18n:check -w sceniter-shared            souhrn
//   npm run i18n:check -w sceniter-shared -- --list  i s klíči navíc

import { checkCatalogs, coreCatalogs } from 'core-shared'
import { sceniterCatalogs } from '../src/i18n'

const list = process.argv.includes('--list')
const ok = [checkCatalogs('jádro', coreCatalogs, { list }), checkCatalogs('sceniter', sceniterCatalogs, { list })]
if (ok.includes(false)) process.exitCode = 1
