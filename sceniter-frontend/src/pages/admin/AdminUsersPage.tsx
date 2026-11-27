import { HistoryIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { queryKeys, type User } from 'sceniter-shared'
import { AdminUsersPage as CoreAdminUsersPage } from 'core-web/src/pages/admin/AdminUsersPage'

// Přehled sledování ukazuje všechny účty se jménem a e-mailem; klíč je
// prefixem i detailu uživatele.
const PRODUCT_KEYS = [queryKeys.adminWatching]

/** Správa účtů z core-web s odkazem na historii sledování uživatele. */
export function AdminUsersPage() {
  const { t } = useTranslation()

  return (
    <CoreAdminUsersPage
      productKeys={PRODUCT_KEYS}
      historyLink={(user: User) => ({
        to: `/admin/watching/${user.id}`,
        label: t('video.admin.watching.historyOf', { email: user.email }),
        icon: HistoryIcon,
      })}
    />
  )
}
