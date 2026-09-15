import { UserPlusIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { useAdminUsers, useDeleteUser } from '@/api/adminHooks'
import type { User } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { ConfirmDialog } from '@/components/admin/ConfirmDialog'
import { CreateUserDialog } from '@/components/admin/CreateUserDialog'
import { ResetPasswordDialog } from '@/components/admin/ResetPasswordDialog'
import { UserTable } from '@/components/admin/UserTable'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { Button } from '@/components/ui/button'

export function AdminUsersPage() {
  const { user: currentUser } = useAuth()
  const users = useAdminUsers()
  const deleteUser = useDeleteUser()

  const [createOpen, setCreateOpen] = useState(false)
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null)

  function handleDelete() {
    if (!deleteTarget) return

    deleteUser.mutate(deleteTarget.id, {
      onSuccess: () => {
        toast.success(`Účet ${deleteTarget.email} byl smazán.`)
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
          Nový uživatel
        </Button>
      </div>

      {users.isPending ? <LoadingList count={4} /> : null}
      {users.error ? <ErrorState error={users.error} onRetry={() => void users.refetch()} /> : null}
      {users.data?.length === 0 ? <EmptyState title="Žádní uživatelé" /> : null}
      {users.data && users.data.length > 0 ? (
        <UserTable
          users={users.data}
          currentUserId={currentUser?.id ?? ''}
          onResetPassword={setResetTarget}
          onDelete={setDeleteTarget}
        />
      ) : null}

      <CreateUserDialog open={createOpen} onOpenChange={setCreateOpen} />
      <ResetPasswordDialog user={resetTarget} onClose={() => setResetTarget(null)} />
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Smazat účet?"
        description={
          <>
            Účet <strong>{deleteTarget?.email}</strong> se smaže i s jeho pozicemi v přehrávání
            a záložkami. Knihy v knihovně to nijak nezmění.
          </>
        }
        confirmLabel="Smazat"
        pendingLabel="Mažu…"
        destructive
        pending={deleteUser.isPending}
        onConfirm={handleDelete}
      />
    </div>
  )
}
