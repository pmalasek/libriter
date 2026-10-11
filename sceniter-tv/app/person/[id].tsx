import { ScrollView, StyleSheet, View } from 'react-native'
import { Image } from 'expo-image'
import { useLocalSearchParams } from 'expo-router'
import { useTranslation } from 'react-i18next'
import { profileUrl, type Movie, type Show } from 'sceniter-shared'
import { usePerson } from '@/api/queries'
import { PosterCard } from '@/components/Cards'
import { Row } from '@/components/Row'
import { ErrorView, Loading } from '@/components/States'
import { T } from '@/components/T'
import { openMovie, openShow } from '@/lib/nav'
import { posterImage } from '@/lib/titles'
import { card, colors, px, safe } from '@/theme'

export default function PersonScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const { t } = useTranslation()
  const person = usePerson(id)
  const p = person.data
  if (!p) return person.isError ? <ErrorView onRetry={() => void person.refetch()} /> : <Loading />

  const movies = p.credits.flatMap((c) => (c.movie ? [c.movie] : []))
  const shows = p.credits.flatMap((c) => (c.show ? [c.show] : []))
  const photo = profileUrl(p, px(260) * 2)

  return (
    <ScrollView contentContainerStyle={{ paddingBottom: px(120) }}>
      <View style={styles.head}>
        <View style={styles.photo}>{photo ? <Image source={photo} style={StyleSheet.absoluteFill} contentFit="cover" /> : null}</View>
        <View>
          <T variant="hero">{p.name}</T>
          <T variant="caption" dim>
            {[
              movies.length ? t('video.person.moviesCount', { count: movies.length }) : null,
              shows.length ? t('video.person.showsCount', { count: shows.length }) : null,
            ]
              .filter(Boolean)
              .join(' · ')}
          </T>
        </View>
      </View>
      <Row<Movie>
        title={t('video.person.movies')}
        items={movies}
        itemWidth={card.posterW}
        keyOf={(m) => m.id}
        render={(m, i) => (
          <PosterCard title={m.title} subtitle={m.year ? String(m.year) : undefined} image={posterImage(m)} hasTVPreferredFocus={i === 0} onPress={() => openMovie(m.id)} />
        )}
      />
      <Row<Show>
        title={t('video.person.shows')}
        items={shows}
        itemWidth={card.posterW}
        keyOf={(s) => s.id}
        render={(s, i) => (
          <PosterCard
            title={s.title}
            subtitle={s.year ? String(s.year) : undefined}
            image={posterImage(s)}
            hasTVPreferredFocus={movies.length === 0 && i === 0}
            onPress={() => openShow(s.id)}
          />
        )}
      />
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  head: { flexDirection: 'row', alignItems: 'center', gap: px(48), paddingHorizontal: safe.x, paddingTop: px(120), paddingBottom: px(56) },
  photo: { width: px(260), height: px(260), borderRadius: px(130), overflow: 'hidden', backgroundColor: colors.surfaceHi },
})
