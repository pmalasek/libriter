import { useEffect, useRef, useState, type ComponentRef } from 'react'
import { ScrollView, StyleSheet, TextInput, TVFocusGuideView, View } from 'react-native'
import { useLocalSearchParams } from 'expo-router'
import { useTranslation } from 'react-i18next'
import { SearchIcon } from 'lucide-react-native'
import { profileUrl, type Movie, type Person, type Show } from 'sceniter-shared'
import { useSearch } from '@/api/queries'
import { PersonCard, PosterCard } from '@/components/Cards'
import { Row } from '@/components/Row'
import { T } from '@/components/T'
import { useDownFromInput } from '@/lib/inputEscape'
import { openMovie, openPerson, openShow } from '@/lib/nav'
import { posterImage } from '@/lib/titles'
import { card, colors, fonts, px, radius, safe } from '@/theme'

export default function SearchScreen() {
  const { t } = useTranslation()
  // Otevřeno tlačítkem Hledat v knihovně: fokus rovnou do pole (otevře klávesnici).
  const { focus } = useLocalSearchParams<{ focus?: string }>()
  const inputRef = useRef<TextInput>(null)
  const focusConsumed = useRef(false)
  useEffect(() => {
    if (focus !== '1' || focusConsumed.current) return
    focusConsumed.current = true
    const timer = setTimeout(() => inputRef.current?.focus(), 50)
    return () => clearTimeout(timer)
  }, [focus])
  const [text, setText] = useState('')
  const [query, setQuery] = useState('')
  const [inputFocused, setInputFocused] = useState(false)
  // Šipka dolů z pole na první výsledek (EditText ji sám nepustí).
  const resultsRef = useRef<ComponentRef<typeof TVFocusGuideView>>(null)
  const escape = useDownFromInput(() => resultsRef.current)
  useEffect(() => {
    const timer = setTimeout(() => setQuery(text.trim()), 300)
    return () => clearTimeout(timer)
  }, [text])
  const results = useSearch(query)
  const r = results.data
  const nothing = query.length > 1 && r && r.movies.length + r.shows.length + r.people.length === 0

  return (
    <View style={styles.root}>
      {/* Fokus sem dá výběr v menu; klávesnice se otevře až po OK (fokus
          z JS by ji na Androidu otevřel hned a ta by pak brala šipky). */}
      <View style={[styles.inputRow, inputFocused && styles.inputRowFocused]}>
        <SearchIcon size={px(40)} color={inputFocused ? colors.accent : colors.textDim} />
        <TextInput
          ref={inputRef}
          value={text}
          onChangeText={setText}
          placeholder={t('video.search.placeholder')}
          placeholderTextColor={colors.textFaint}
          autoCorrect={false}
          returnKeyType="search"
          onFocus={() => {
            setInputFocused(true)
            escape.onFocus()
          }}
          onBlur={() => {
            setInputFocused(false)
            escape.onBlur()
          }}
          style={styles.input}
        />
      </View>
      <ScrollView contentContainerStyle={{ paddingBottom: px(120) }} showsVerticalScrollIndicator={false}>
        {query.length < 2 ? (
          <T dim style={styles.hint}>
            {t('video.search.empty')}
          </T>
        ) : nothing ? (
          <T dim style={styles.hint}>
            {t('video.search.noResults', { query })}
          </T>
        ) : null}
        <TVFocusGuideView ref={resultsRef} autoFocus>
          <Row<Movie>
            title={t('video.movies.title')}
            items={r?.movies ?? []}
            itemWidth={card.posterW}
            keyOf={(m) => m.id}
            render={(m) => <PosterCard title={m.title} subtitle={m.year ? String(m.year) : undefined} image={posterImage(m)} onPress={() => openMovie(m.id)} />}
          />
          <Row<Show>
            title={t('video.shows.title')}
            items={r?.shows ?? []}
            itemWidth={card.posterW}
            keyOf={(s) => s.id}
            render={(s) => <PosterCard title={s.title} subtitle={s.year ? String(s.year) : undefined} image={posterImage(s)} onPress={() => openShow(s.id)} />}
          />
          <Row<Person>
            title={t('video.search.people')}
            items={r?.people ?? []}
            itemWidth={px(200)}
            keyOf={(p) => p.id}
            render={(p) => <PersonCard title={p.name} image={profileUrl(p, px(170) * 2)} onPress={() => openPerson(p.id)} />}
          />
        </TVFocusGuideView>
      </ScrollView>
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1, paddingTop: safe.y },
  inputRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: px(20),
    marginHorizontal: safe.x,
    marginBottom: px(36),
    paddingHorizontal: px(28),
    height: px(100),
    borderRadius: radius.button,
    backgroundColor: 'rgba(255,255,255,0.1)',
    borderWidth: px(4),
    borderColor: 'transparent',
  },
  inputRowFocused: { borderColor: colors.focus, backgroundColor: 'rgba(255,255,255,0.16)' },
  input: { flex: 1, color: colors.text, fontFamily: fonts.bodyMedium, fontSize: px(38) },
  hint: { marginHorizontal: safe.x, marginBottom: px(32) },
})
