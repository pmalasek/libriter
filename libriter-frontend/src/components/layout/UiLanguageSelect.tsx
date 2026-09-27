import { useTranslation } from 'react-i18next'
import { isUILanguage, UI_LANGUAGE_NAMES, UI_LANGUAGES } from '@/api/types'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useLanguage } from '@/i18n/language'

/** Výběr jazyka rozhraní s viditelnou aktuální volbou (profil). */
export function UiLanguageSelect({ id }: { id?: string }) {
  const { t } = useTranslation()
  const { language, setLanguage, isSaving } = useLanguage()

  return (
    <Select
      value={language}
      onValueChange={(value) => isUILanguage(value) && setLanguage(value)}
      disabled={isSaving}
    >
      <SelectTrigger id={id} aria-label={t('language.label')} aria-busy={isSaving} className="min-w-44">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {UI_LANGUAGES.map((code) => (
          <SelectItem key={code} value={code} lang={code}>
            {UI_LANGUAGE_NAMES[code]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
