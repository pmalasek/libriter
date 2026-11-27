import { KeyRoundIcon, PencilIcon, Trash2Icon, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { formatDate, roleLabel, type Role, type User } from 'core-shared'
import { useSetUserRole, type ProductKeys } from '../../api/hooks'
import { Button } from '../ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../ui/table'

const ROLES: Role[] = ['reader', 'editor', 'admin']

/**
 * Odkaz na historii uživatele v produktu (poslechy v Libriteru, sledování ve
 * Sceniteru). Jádro historii nezná, ikonu i adresu dodá produkt.
 */
export interface UserHistoryLink {
  to: string
  /** Popisek pro čtečky, např. „Poslechy uživatele …“. */
  label: string
  icon: LucideIcon
}

export function UserTable({
  users,
  currentUserId,
  onEdit,
  onResetPassword,
  onDelete,
  historyLink,
  productKeys,
}: {
  users: User[]
  currentUserId: string
  onEdit: (user: User) => void
  onResetPassword: (user: User) => void
  onDelete: (user: User) => void
  /** Volitelný odkaz na historii uživatele; bez něj se tlačítko nezobrazí. */
  historyLink?: (user: User) => UserHistoryLink
  productKeys?: ProductKeys
}) {
  const { t } = useTranslation()
  const setRole = useSetUserRole(productKeys)

  function handleRoleChange(user: User, role: Role) {
    if (role === user.role) return

    setRole.mutate(
      { userId: user.id, role },
      {
        onSuccess: () => toast.success(
            t('admin.users.table.roleChanged', { email: user.email, role: roleLabel(role) }),
          ),
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <div className="rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('admin.users.fields.name')}</TableHead>
            <TableHead>{t('admin.users.fields.email')}</TableHead>
            <TableHead>{t('admin.users.fields.login')}</TableHead>
            <TableHead className="w-44">{t('admin.users.fields.role')}</TableHead>
            <TableHead className="w-36">{t('admin.users.table.created')}</TableHead>
            <TableHead className="w-40 text-right">{t('admin.users.table.actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => {
            // Vlastní účet si admin nemůže degradovat ani smazat – server to
            // stejně odmítne a tlačítko by jen mátlo.
            const isSelf = user.id === currentUserId
            const history = historyLink?.(user)

            return (
              <TableRow key={user.id}>
                <TableCell className="font-medium">
                  {user.display_name}
                  {isSelf ? (
                    <span className="ml-2 text-xs text-muted-foreground">
                      {t('admin.users.table.you')}
                    </span>
                  ) : null}
                </TableCell>
                <TableCell className="break-all">{user.email}</TableCell>
                <TableCell className="break-all">
                  {user.login || <span className="text-muted-foreground">—</span>}
                </TableCell>
                <TableCell>
                  <Select
                    value={user.role}
                    disabled={isSelf || setRole.isPending}
                    onValueChange={(value) => handleRoleChange(user, value as Role)}
                  >
                    <SelectTrigger className="w-full" aria-label={t('admin.users.table.roleOf', { email: user.email })}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {ROLES.map((role) => (
                        <SelectItem key={role} value={role}>
                          {roleLabel(role)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {formatDate(user.created_at)}
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-1">
                    {history ? (
                      <Button asChild variant="ghost" size="icon">
                        <Link to={history.to} aria-label={history.label}>
                          <history.icon />
                        </Link>
                      </Button>
                    ) : null}
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={t('admin.users.table.editAccount', { email: user.email })}
                      onClick={() => onEdit(user)}
                    >
                      <PencilIcon />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={t('admin.users.table.resetPassword', { email: user.email })}
                      onClick={() => onResetPassword(user)}
                    >
                      <KeyRoundIcon />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={t('admin.users.table.deleteAccount', { email: user.email })}
                      disabled={isSelf}
                      onClick={() => onDelete(user)}
                    >
                      <Trash2Icon />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
