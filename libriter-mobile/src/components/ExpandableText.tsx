import { useState } from 'react'
import { Pressable, View, type NativeSyntheticEvent, type TextLayoutEventData } from 'react-native'
import { useTranslation } from 'react-i18next'

import { fonts, spacing, useTheme } from '@/theme'
import { Body } from './ui/Text'

/**
 * Text zkrácený na pár řádků s tlačítkem „Zobrazit více“ – jen když opravdu
 * přetéká. Popisy knih bývají dlouhé a bez zkrácení by odsunuly kapitoly
 * pod okraj obrazovky.
 */
export function ExpandableText({ text, lines = 4 }: { text: string; lines?: number }) {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const [open, setOpen] = useState(false)
  const [overflows, setOverflows] = useState(false)

  const onLayout = (event: NativeSyntheticEvent<TextLayoutEventData>) => {
    if (!open && event.nativeEvent.lines.length > lines) setOverflows(true)
  }

  return (
    <View style={{ gap: spacing.sm }}>
      <Body size={14} numberOfLines={open ? undefined : lines} onTextLayout={onLayout} style={{ color: colors.mutedForeground }}>
        {text}
      </Body>
      {overflows ? (
        <Pressable onPress={() => setOpen((value) => !value)} hitSlop={8}>
          <Body size={13} style={{ color: colors.primary, fontFamily: fonts.sansMedium }}>
            {open ? t('mobile.expandableText.less') : t('mobile.expandableText.more')}
          </Body>
        </Pressable>
      ) : null}
    </View>
  )
}
