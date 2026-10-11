import { useLocalSearchParams } from 'expo-router'
import { useTranslation } from 'react-i18next'
import { useLibraries } from '@/api/queries'
import { LibraryGrid } from '@/components/LibraryGrid'
import { Empty, ErrorView, Loading } from '@/components/States'

/** Jedna knihovna z bočního menu. */
export default function LibraryScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const { t } = useTranslation()
  const libraries = useLibraries()
  const lib = libraries.data?.find((l) => l.id === id)
  if (!lib) {
    if (libraries.isError) return <ErrorView onRetry={() => void libraries.refetch()} />
    if (libraries.isPending || libraries.isPlaceholderData) return <Loading />
    return <Empty title={t('video.tv.libraryMissing')} />
  }
  // key: přepnutí knihovny založí výpis znovu (vlastní řazení a pozice).
  return <LibraryGrid key={lib.id} library={lib} />
}
