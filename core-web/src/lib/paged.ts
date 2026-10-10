import { keepPreviousData, useQueries, type QueryKey } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Page } from 'core-shared'

/** Výchozí velikost stránky – desítky dlaždic na obrazovku a rezerva. */
export const PAGE_SIZE = 100

/**
 * Jak dlouho drží cache stránku, kterou už nikdo nezobrazuje. Krátce: při
 * rolování zpět se stránka vezme z cache, ale stovky prošlých stránek
 * velké knihovny v paměti nezůstanou.
 */
export const PAGE_GC_TIME = 60_000

export interface PagedListOptions<T> {
  /** Klíč výpisu včetně filtrů a řazení; stránky dostanou vlastní podklíče. */
  queryKey: QueryKey
  fetchPage: (offset: number, limit: number, signal: AbortSignal) => Promise<Page<T>>
  pageSize?: number
  enabled?: boolean
}

export interface PagedList<T> {
  /** Počet všech položek; undefined, dokud nepřišla první stránka. */
  total: number | undefined
  /** Položka na indexu; undefined, dokud se její stránka nenačetla. */
  getItem: (index: number) => T | undefined
  /** Nahlásí viditelný rozsah (indexy položek) – dotáhnou se stránky, které ho pokrývají. */
  setRange: (first: number, last: number) => void
  isPending: boolean
  isError: boolean
  error: Error | null
  /** Zobrazená data patří předchozímu filtru, nový se teprve načítá. */
  isPlaceholderData: boolean
  refetch: () => void
}

/**
 * Stránkovaný výpis s náhodným přístupem: drží jen stránky, které pokrývají
 * viditelnou část seznamu (a první stránku kvůli celkovému počtu). Seznam
 * může mít desítky tisíc položek – v paměti jsou vždy jen desítky.
 */
export function usePagedList<T>({
  queryKey,
  fetchPage,
  pageSize = PAGE_SIZE,
  enabled = true,
}: PagedListOptions<T>): PagedList<T> {
  const keyHash = JSON.stringify(queryKey)
  const [range, setRangeState] = useState<{ key: string; first: number; last: number }>({
    key: keyHash,
    first: 0,
    last: 0,
  })
  // Nový filtr začíná od začátku seznamu.
  const current = range.key === keyHash ? range : { key: keyHash, first: 0, last: 0 }

  const pages = useMemo(() => {
    const from = Math.floor(current.first / pageSize)
    const to = Math.floor(current.last / pageSize)
    const list = [0]
    for (let p = Math.max(from, 1); p <= to; p++) list.push(p)
    return list
  }, [current.first, current.last, pageSize])

  const fetchRef = useRef(fetchPage)
  useEffect(() => {
    fetchRef.current = fetchPage
  })

  const queries = useQueries({
    queries: pages.map((page) => ({
      queryKey: [...queryKey, 'page', pageSize, page],
      queryFn: ({ signal }: { signal: AbortSignal }) => fetchRef.current(page * pageSize, pageSize, signal),
      enabled,
      gcTime: PAGE_GC_TIME,
      // Při změně filtru zůstane vidět předchozí výsledek, dokud nedorazí nový.
      placeholderData: page === 0 ? keepPreviousData : undefined,
    })),
  })

  const first = queries[0]
  const total = first?.data?.total ?? queries.find((q) => q.data)?.data?.total

  const byPage = new Map<number, Page<T>>()
  pages.forEach((page, i) => {
    const data = queries[i]?.data
    if (data) byPage.set(page, data)
  })
  // Mapu stránek drží ref, aby getItem měl stálou identitu jen podle dat.
  const dataKey = queries.map((q) => q.dataUpdatedAt).join(',') + '|' + pages.join(',')
  const getItem = useMemo(() => {
    const snapshot = byPage
    return (index: number) => snapshot.get(Math.floor(index / pageSize))?.items[index % pageSize]
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dataKey, pageSize])

  const setRange = useCallback(
    (from: number, to: number) => {
      setRangeState((prev) => {
        const lo = Math.max(0, from)
        const hi = Math.max(lo, to)
        if (prev.key === keyHash && prev.first === lo && prev.last === hi) return prev
        return { key: keyHash, first: lo, last: hi }
      })
    },
    [keyHash],
  )

  // Při změně filtru (psaní do hledání) zůstává vidět poslední výsledek,
  // dokud nedorazí první stránka nového – seznam neproblikne na kostru
  // a nic se neodpojí (vyhledávací pole by ztratilo fokus).
  const lastGood = useRef<{ total: number; getItem: (index: number) => T | undefined } | null>(null)
  const fresh = total !== undefined && !first?.isPlaceholderData
  useEffect(() => {
    if (fresh) lastGood.current = { total: total!, getItem }
  })
  if (total === undefined && lastGood.current && !queries.some((q) => q.isError)) {
    return {
      total: lastGood.current.total,
      getItem: lastGood.current.getItem,
      setRange,
      isPending: false,
      isError: false,
      error: null,
      isPlaceholderData: true,
      refetch: () => {
        for (const q of queries) void q.refetch()
      },
    }
  }

  return {
    total,
    getItem,
    setRange,
    isPending: Boolean(first?.isPending),
    isError: queries.some((q) => q.isError),
    error: queries.find((q) => q.error)?.error ?? null,
    isPlaceholderData: Boolean(first?.isPlaceholderData),
    refetch: () => {
      for (const q of queries) void q.refetch()
    },
  }
}

/** Hodnota, která se změní až po `delay` ms klidu – hledání se neposílá po každé klávese. */
export function useDebouncedValue<T>(value: T, delay = 250): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(id)
  }, [value, delay])
  return debounced
}
