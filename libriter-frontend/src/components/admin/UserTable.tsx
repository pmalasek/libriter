import { HeadphonesIcon, KeyRoundIcon, Trash2Icon } from 'lucide-react'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { useSetUserRole } from '@/api/adminHooks'
import { ROLE_LABELS, type Role, type User } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatDate } from '@/lib/format'

const ROLES: Role[] = ['reader', 'editor', 'admin']

export function UserTable({
  users,
  currentUserId,
  onResetPassword,
  onDelete,
}: {
  users: User[]
  currentUserId: string
  onResetPassword: (user: User) => void
  onDelete: (user: User) => void
}) {
  const setRole = useSetUserRole()

  function handleRoleChange(user: User, role: Role) {
    if (role === user.role) return

    setRole.mutate(
      { userId: user.id, role },
      {
        onSuccess: () => toast.success(`${user.email} má nově roli ${ROLE_LABELS[role]}.`),
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <div className="rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Jméno</TableHead>
            <TableHead>E-mail</TableHead>
            <TableHead className="w-44">Role</TableHead>
            <TableHead className="w-36">Vytvořen</TableHead>
            <TableHead className="w-32 text-right">Akce</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => {
            // Vlastní účet si admin nemůže degradovat ani smazat – server to
            // stejně odmítne a tlačítko by jen mátlo.
            const isSelf = user.id === currentUserId

            return (
              <TableRow key={user.id}>
                <TableCell className="font-medium">
                  {user.display_name}
                  {isSelf ? <span className="ml-2 text-xs text-muted-foreground">(vy)</span> : null}
                </TableCell>
                <TableCell className="break-all">{user.email}</TableCell>
                <TableCell>
                  <Select
                    value={user.role}
                    disabled={isSelf || setRole.isPending}
                    onValueChange={(value) => handleRoleChange(user, value as Role)}
                  >
                    <SelectTrigger className="w-full" aria-label={`Role uživatele ${user.email}`}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {ROLES.map((role) => (
                        <SelectItem key={role} value={role}>
                          {ROLE_LABELS[role]}
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
                    <Button asChild variant="ghost" size="icon">
                      <Link
                        to={`/admin/listening/${user.id}`}
                        aria-label={`Poslechy uživatele ${user.email}`}
                      >
                        <HeadphonesIcon />
                      </Link>
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Reset hesla účtu ${user.email}`}
                      onClick={() => onResetPassword(user)}
                    >
                      <KeyRoundIcon />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Smazat účet ${user.email}`}
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
