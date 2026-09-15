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
import { cn } from '@/lib/utils'

/**
 * Účet v hlavičce (jen avatar) nebo v patičce sidebaru (showName – avatar
 * se jménem a rolí, roztažený na celou šířku).
 */
export function UserMenu({ showName = false, className }: { showName?: boolean; className?: string }) {
  const { user, signOut } = useAuth()
  const navigate = useNavigate()

  if (!user) return null

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        {showName ? (
          <Button
            variant="ghost"
            size="lg"
            aria-label="Uživatelské menu"
            className={cn('h-auto min-w-0 flex-1 justify-start gap-2 px-1.5 py-1.5', className)}
          >
            <Avatar className="size-8">
              <AvatarFallback className="text-xs">{initials(user.display_name)}</AvatarFallback>
            </Avatar>
            <span className="min-w-0 text-left">
              <span className="block truncate text-sm font-medium">{user.display_name}</span>
              <span className="block truncate text-xs font-normal text-muted-foreground">
                {ROLE_LABELS[user.role] ?? user.role}
              </span>
            </span>
          </Button>
        ) : (
          <Button variant="ghost" size="icon" aria-label="Uživatelské menu" className={className}>
            <Avatar className="size-7">
              <AvatarFallback className="text-xs">{initials(user.display_name)}</AvatarFallback>
            </Avatar>
          </Button>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56" sideOffset={8}>
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
