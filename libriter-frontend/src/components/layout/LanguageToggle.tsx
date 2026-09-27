import { LanguagesIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { isUILanguage, UI_LANGUAGE_NAMES, UI_LANGUAGES } from '@/api/types'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useLanguage } from '@/i18n/language'
import { cn } from '@/lib/utils'

export function LanguageToggle({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { language, setLanguage, isSaving } = useLanguage()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t('language.label')}
          title={isSaving ? t('language.saving') : t('language.label')}
          aria-busy={isSaving}
          className={cn('shrink-0', className)}
        >
          <LanguagesIcon className={isSaving ? 'animate-pulse' : undefined} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuLabel>{t('language.label')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={language}
          onValueChange={(value) => isUILanguage(value) && setLanguage(value)}
          aria-label={t('language.label')}
        >
          {UI_LANGUAGES.map((code) => (
            <DropdownMenuRadioItem key={code} value={code} disabled={isSaving} lang={code}>
              {UI_LANGUAGE_NAMES[code]}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
