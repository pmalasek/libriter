import { LogOutIcon, UserIcon } from 'lucide-react'
import { useNavigate } from 'react-router'
import { ROLE_LABELS } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { initials } from '@/lib/format'

export function UserMenu() {
  const { user, signOut } = useAuth()
  const navigate = useNavigate()

  if (!user) return null

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Uživatelské menu">
          <Avatar className="size-7">
            <AvatarFallback className="text-xs">{initials(user.display_name)}</AvatarFallback>
          </Avatar>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel>
          <div className="truncate font-medium">{user.display_name}</div>
          <div className="truncate text-xs font-normal text-muted-foreground">{user.email}</div>
          <div className="mt-1 text-xs font-normal text-muted-foreground">
            {ROLE_LABELS[user.role] ?? user.role}
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => navigate('/profile')}>
          <UserIcon />
          Profil
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={() => {
            signOut()
            navigate('/login', { replace: true })
          }}
        >
          <LogOutIcon />
          Odhlásit se
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
