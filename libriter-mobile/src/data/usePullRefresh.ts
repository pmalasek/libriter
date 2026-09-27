import { useCallback, useState } from 'react'

import { syncEngine } from '@/sync/syncEngine'

/**
 * Tažení dolů pro obnovení seznamu. Nejdřív proběhne synchronizace – odešle
 * frontu pozic a v offline režimu obnoví zrcadlo, ze kterého se pak čte –
 * a teprve potom se znovu načte obrazovka.
 */
export function usePullRefresh(refetch: () => Promise<unknown>) {
  const [refreshing, setRefreshing] = useState(false)

  const onRefresh = useCallback(() => {
    setRefreshing(true)
    void syncEngine
      .syncNow()
      .then(() => refetch())
      .finally(() => setRefreshing(false))
  }, [refetch])

  return { refreshing, onRefresh }
}
