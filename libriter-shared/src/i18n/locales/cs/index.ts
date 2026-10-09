import type { Catalog } from 'core-shared'
import type en from '../en'
import books from './books'
import authors from './authors'
import series from './series'
import sessions from './sessions'
import player from './player'
import mobile from './mobile'

const cs: Catalog<typeof en> = {
  books,
  authors,
  series,
  sessions,
  player,
  mobile,
}

export default cs
