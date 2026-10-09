// Přehled úplnosti překladů Libriteru (a jádra) proti angličtině.
//
//   npm run i18n:check -w libriter-shared            souhrn
//   npm run i18n:check -w libriter-shared -- --list  i s klíči navíc

import { checkCatalogs, coreCatalogs } from 'core-shared'
import { libriterCatalogs } from '../src/i18n'

const list = process.argv.includes('--list')
const ok = [checkCatalogs('jádro', coreCatalogs, { list }), checkCatalogs('libriter', libriterCatalogs, { list })]
if (ok.includes(false)) process.exitCode = 1
