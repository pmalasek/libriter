import { UserPlusIcon } from 'lucide-react'
import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAdminUsers, useDeleteUser } from '@/api/adminHooks'
import type { User } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { ConfirmDialog } from '@/components/admin/ConfirmDialog'
import { CreateUserDialog } from '@/components/admin/CreateUserDialog'
import { EditUserDialog } from '@/components/admin/EditUserDialog'
import { ResetPasswordDialog } from '@/components/admin/ResetPasswordDialog'
import { UserTable } from '@/components/admin/UserTable'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { Button } from '@/components/ui/button'

export function AdminUsersPage() {
  const { t } = useTranslation()
  const { user: currentUser } = useAuth()
  const users = useAdminUsers()
  const deleteUser = useDeleteUser()

  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<User | null>(null)
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null)

  function handleDelete() {
    if (!deleteTarget) return

    deleteUser.mutate(deleteTarget.id, {
      onSuccess: () => {
        toast.success(t('admin.users.deleted', { email: deleteTarget.email }))
        setDeleteTarget(null)
      },
      onError: (error) => {
        toast.error(error.message)
        setDeleteTarget(null)
      },
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button onClick={() => setCreateOpen(true)}>
          <UserPlusIcon />
          {t('admin.users.newUser')}
        </Button>
      </div>

      {users.isPending ? <LoadingList count={4} /> : null}
      {users.error ? <ErrorState error={users.error} onRetry={() => void users.refetch()} /> : null}
      {users.data?.length === 0 ? <EmptyState title={t('admin.users.empty')} /> : null}
      {users.data && users.data.length > 0 ? (
        <UserTable
          users={users.data}
          currentUserId={currentUser?.id ?? ''}
          onEdit={setEditTarget}
          onResetPassword={setResetTarget}
          onDelete={setDeleteTarget}
        />
      ) : null}

      <CreateUserDialog open={createOpen} onOpenChange={setCreateOpen} />
      <EditUserDialog user={editTarget} onClose={() => setEditTarget(null)} />
      <ResetPasswordDialog user={resetTarget} onClose={() => setResetTarget(null)} />
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('admin.users.deleteTitle')}
        description={
          <Trans
            i18nKey="admin.users.deleteDescription"
            values={{ email: deleteTarget?.email }}
            components={{ strong: <strong /> }}
          />
        }
        confirmLabel={t('common.delete')}
        pendingLabel={t('admin.users.deleting')}
        destructive
        pending={deleteUser.isPending}
        onConfirm={handleDelete}
      />
    </div>
  )
}
