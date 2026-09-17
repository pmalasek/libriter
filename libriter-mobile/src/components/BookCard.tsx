import { memo } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Check, CheckCircle2, Headphones } from 'lucide-react-native'
import { authorNames, BOOK_STATUS_LABELS, formatDuration, type Book, type BookStatus } from 'libriter-shared'

import { radius, spacing, useTheme } from '@/theme'
import { BookCover } from './BookCover'
import { Muted, Title } from './ui/Text'

/** Ovládání hromadného výběru; když chybí, karta je odkaz na detail. */
export interface CardSelection {
  selected: boolean
  onToggle: () => void
}

/** Zaškrtávací značka v rohu obálky v režimu výběru. */
function SelectionMark({ selected }: { selected: boolean }) {
  const { colors } = useTheme()
  return (
    <View
      style={[
        styles.mark,
        styles.markSquare,
        selected
          ? { backgroundColor: colors.primary, borderColor: colors.primary }
          : { backgroundColor: colors.glassStrong, borderColor: 'rgba(255,255,255,0.6)' },
      ]}
    >
      {selected ? <Check color={colors.primaryForeground} size={15} strokeWidth={3} /> : null}
    </View>
  )
}

/**
 * Značka poslechu: doposlechnutá kniha má fajfku, rozposlouchaná sluchátka.
 * U neposlechnuté se nevykresluje nic – nepoznaná kniha je většina knihovny
 * a značka u každé dlaždice by ztratila smysl.
 */
export function BookStatusMark({ status, size = 24 }: { status: BookStatus; size?: number }) {
  const { colors } = useTheme()
  if (status === 'none') return null
  const finished = status === 'finished'
  const Icon = finished ? CheckCircle2 : Headphones
  return (
    <View
      accessibilityLabel={BOOK_STATUS_LABELS[status]}
      style={[
        styles.mark,
        { width: size, height: size, borderRadius: size / 2, backgroundColor: finished ? colors.primary : colors.glassStrong },
      ]}
    >
      <Icon color={finished ? colors.primaryForeground : colors.primary} size={size * 0.62} strokeWidth={2.2} />
    </View>
  )
}

/** Dlaždice knihy – velká (výchozí) nebo malá bez délky. */
export const BookCard = memo(function BookCard({
  book,
  size = 'tiles',
  series,
  selection,
  status = 'none',
  downloaded = false,
}: {
  book: Book
  size?: 'tiles' | 'small'
  /** Popisek série („Atomové šelmy · 2. díl“); prázdný u knihy mimo sérii. */
  series?: string
  selection?: CardSelection
  status?: BookStatus
  downloaded?: boolean
}) {
  const { colors } = useTheme()
  const router = useRouter()
  const small = size === 'small'
  const meta = (small ? [book.published_year] : [book.published_year, formatDuration(book.duration_seconds)])
    .filter(Boolean)
    .join(' · ')

  return (
    <Pressable
      onPress={() => (selection ? selection.onToggle() : router.push(`/book/${book.id}`))}
      accessibilityRole="button"
      accessibilityState={selection ? { selected: selection.selected } : undefined}
      style={({ pressed }) => ({ opacity: pressed ? 0.7 : 1 })}
    >
      <View>
        <BookCover
          book={book}
          downloaded={downloaded}
          style={selection?.selected ? { borderWidth: 3, borderColor: colors.primary } : undefined}
        />
        {selection ? (
          <View style={styles.topLeft}>
            <SelectionMark selected={selection.selected} />
          </View>
        ) : null}
        {/* Výběr sedí vlevo, stav poslechu tedy vpravo – nepřekrývají se. */}
        <View style={styles.topRight}>
          <BookStatusMark status={status} />
        </View>
      </View>
      <View style={{ marginTop: small ? spacing.sm : spacing.sm + 4, gap: 2 }}>
        <Title numberOfLines={2} size={small ? 12 : 14}>
          {book.title}
        </Title>
        <Muted numberOfLines={1} size={small ? 11 : 12}>
          {authorNames(book.authors)}
        </Muted>
        {series ? (
          <Muted numberOfLines={1} size={small ? 11 : 12} style={{ color: colors.primary, opacity: 0.85 }}>
            {series}
          </Muted>
        ) : null}
        {/* Malá dlaždice má málo místa, vejde se jen rok prvního vydání. */}
        {meta ? <Muted size={small ? 11 : 12}>{meta}</Muted> : null}
      </View>
    </Pressable>
  )
})

/** Hlavička seznamu knih – sloupce kopírují BookRow. */
export function BookRowHeader({ selecting = false }: { selecting?: boolean }) {
  const { colors } = useTheme()
  return (
    <View style={[styles.row, { backgroundColor: colors.muted, paddingVertical: spacing.sm }]}>
      {selecting ? <View style={{ width: 24 }} /> : null}
      <View style={{ width: 48 }} />
      <Muted size={11} style={[styles.header, { flex: 1 }]}>
        Název, autor a série
      </Muted>
      <Muted size={11} style={[styles.header, styles.colYear]}>
        Vydáno
      </Muted>
      <Muted size={11} style={[styles.header, styles.colDuration]}>
        Délka
      </Muted>
    </View>
  )
}

/** Řádek knihy v seznamovém zobrazení. */
export const BookRow = memo(function BookRow({
  book,
  series,
  selection,
  status = 'none',
  downloaded = false,
}: {
  book: Book
  series?: string
  selection?: CardSelection
  status?: BookStatus
  downloaded?: boolean
}) {
  const { colors } = useTheme()
  const router = useRouter()

  return (
    <Pressable
      onPress={() => (selection ? selection.onToggle() : router.push(`/book/${book.id}`))}
      accessibilityRole="button"
      style={({ pressed }) => [
        styles.row,
        { backgroundColor: selection?.selected ? colors.accent : pressed ? colors.muted : 'transparent' },
      ]}
    >
      {selection ? <SelectionMark selected={selection.selected} /> : null}
      <View>
        <BookCover book={book} size={48} rounded={radius.md} downloaded={downloaded} />
        <View style={{ position: 'absolute', top: -4, right: -4 }}>
          <BookStatusMark status={status} size={20} />
        </View>
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Title numberOfLines={1}>{book.title}</Title>
        <Muted numberOfLines={1} size={12}>
          {[authorNames(book.authors), series].filter(Boolean).join(' · ')}
        </Muted>
      </View>
      <Muted size={12} style={styles.colYear}>
        {book.published_year ?? ''}
      </Muted>
      <Muted size={12} style={styles.colDuration}>
        {formatDuration(book.duration_seconds)}
      </Muted>
    </Pressable>
  )
})

const styles = StyleSheet.create({
  mark: { alignItems: 'center', justifyContent: 'center' },
  markSquare: { width: 24, height: 24, borderRadius: 6, borderWidth: 2 },
  topLeft: { position: 'absolute', top: 8, left: 8 },
  topRight: { position: 'absolute', top: 8, right: 8 },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm + 4,
    paddingHorizontal: spacing.sm + 4,
    paddingVertical: spacing.sm,
  },
  header: { textTransform: 'uppercase', letterSpacing: 0.8 },
  colYear: { width: 44, textAlign: 'right' },
  colDuration: { width: 56, textAlign: 'right' },
})
