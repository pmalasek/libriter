import type { Catalog } from '../../catalog'
import type en from '../en'
import common from './common'
import format from './format'
import labels from './labels'
import errors from './errors'
import language from './language'
import auth from './auth'
import layout from './layout'
import profile from './profile'
import books from './books'
import authors from './authors'
import series from './series'
import sessions from './sessions'
import player from './player'
import admin from './admin'
import mobile from './mobile'

const es: Catalog<typeof en> = {
  common,
  format,
  labels,
  errors,
  language,
  auth,
  layout,
  profile,
  books,
  authors,
  series,
  sessions,
  player,
  admin,
  mobile,
}

export default es
