import { HeadphonesIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AdminUsersPage as CoreAdminUsersPage } from 'core-web/src/pages/admin/AdminUsersPage'
import { adminKeys } from '@/api/adminHooks'
import type { User } from '@/api/types'

// Statistiky (počet uživatelů) a přehled poslechů ukazují účty, takže je
// po založení, úpravě a smazání účtu zestaráme spolu se seznamem.
const PRODUCT_KEYS = [adminKeys.stats, adminKeys.listening]

/** Správa účtů z core-web s odkazem na poslechy uživatele. */
export function AdminUsersPage() {
  const { t } = useTranslation()

  return (
    <CoreAdminUsersPage
      productKeys={PRODUCT_KEYS}
      historyLink={(user: User) => ({
        to: `/admin/listening/${user.id}`,
        label: t('admin.users.table.listeningOf', { email: user.email }),
        icon: HeadphonesIcon,
      })}
    />
  )
}
