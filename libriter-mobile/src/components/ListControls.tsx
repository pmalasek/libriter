import { useState } from 'react'
import { Modal, Pressable, StyleSheet, View } from 'react-native'
import { ArrowDownNarrowWide, ArrowUpNarrowWide, ChevronDown, Grid3x3, LayoutGrid, List } from 'lucide-react-native'
import { VIEW_MODE_LABELS, VIEW_MODES, type SortDir, type ViewMode } from 'libriter-shared'

import { fonts, radius, spacing, useTheme } from '@/theme'
import { Body, Muted } from './ui/Text'

const VIEW_ICONS = { tiles: LayoutGrid, small: Grid3x3, list: List } as const

/** Pilulka se třemi ikonami zobrazení – jako `ViewModeToggle` na webu. */
export function ViewModeToggle({ value, onChange }: { value: ViewMode; onChange: (view: ViewMode) => void }) {
  const { colors } = useTheme()
  return (
    <View style={[styles.pill, { backgroundColor: colors.muted }]}>
      {VIEW_MODES.map((mode) => {
        const Icon = VIEW_ICONS[mode]
        const active = mode === value
        return (
          <Pressable
            key={mode}
            onPress={() => onChange(mode)}
            accessibilityRole="button"
            accessibilityLabel={VIEW_MODE_LABELS[mode]}
            accessibilityState={{ selected: active }}
            style={[styles.pillItem, active && { backgroundColor: colors.card }]}
          >
            <Icon color={active ? colors.primary : colors.mutedForeground} size={17} />
          </Pressable>
        )
      })}
    </View>
  )
}

/** Volba klíče řazení (nabídka) + tlačítko směru – jako `SortControl` na webu. */
export function SortControl<K extends string>({
  options,
  value,
  onChange,
  dir,
  onDirChange,
}: {
  options: { value: K; label: string }[]
  value: K
  onChange: (key: K) => void
  dir: SortDir
  onDirChange: (dir: SortDir) => void
}) {
  const { colors } = useTheme()
  const [open, setOpen] = useState(false)
  const current = options.find((option) => option.value === value)
  const DirIcon = dir === 'asc' ? ArrowDownNarrowWide : ArrowUpNarrowWide

  return (
    <View style={styles.sortRow}>
      <Pressable
        onPress={() => setOpen(true)}
        style={[styles.select, { backgroundColor: colors.card, borderColor: colors.border }]}
        accessibilityRole="button"
        accessibilityLabel="Řazení"
      >
        <Body size={14} numberOfLines={1} style={{ flex: 1 }}>
          {current?.label ?? 'Řazení'}
        </Body>
        <ChevronDown color={colors.mutedForeground} size={16} />
      </Pressable>
      <Pressable
        onPress={() => onDirChange(dir === 'asc' ? 'desc' : 'asc')}
        style={[styles.dir, { backgroundColor: colors.card, borderColor: colors.border }]}
        accessibilityRole="button"
        accessibilityLabel={dir === 'asc' ? 'Vzestupně' : 'Sestupně'}
      >
        <DirIcon color={colors.foreground} size={17} />
      </Pressable>

      <Modal visible={open} transparent animationType="fade" onRequestClose={() => setOpen(false)}>
        <Pressable style={styles.backdrop} onPress={() => setOpen(false)}>
          <View style={[styles.sheet, { backgroundColor: colors.card, borderColor: colors.border }]}>
            <Muted size={12} style={{ paddingHorizontal: spacing.md, paddingBottom: spacing.xs }}>
              Řadit podle
            </Muted>
            {options.map((option) => {
              const active = option.value === value
              return (
                <Pressable
                  key={option.value}
                  onPress={() => {
                    onChange(option.value)
                    setOpen(false)
                  }}
                  style={[styles.option, active && { backgroundColor: colors.accent }]}
                >
                  <Body size={15} style={active ? { color: colors.primary, fontFamily: fonts.sansMedium } : undefined}>
                    {option.label}
                  </Body>
                </Pressable>
              )
            })}
          </View>
        </Pressable>
      </Modal>
    </View>
  )
}

const styles = StyleSheet.create({
  pill: { flexDirection: 'row', borderRadius: 999, padding: 3, alignSelf: 'flex-start' },
  pillItem: { width: 40, height: 34, borderRadius: 999, alignItems: 'center', justifyContent: 'center' },
  sortRow: { flexDirection: 'row', gap: spacing.sm, flex: 1 },
  select: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    height: 40,
    borderRadius: radius.lg,
    borderWidth: 1,
    paddingHorizontal: spacing.md - 4,
  },
  dir: { width: 40, height: 40, borderRadius: radius.lg, borderWidth: 1, alignItems: 'center', justifyContent: 'center' },
  backdrop: { flex: 1, backgroundColor: '#00000088', justifyContent: 'flex-end' },
  sheet: {
    borderTopLeftRadius: radius['4xl'],
    borderTopRightRadius: radius['4xl'],
    borderWidth: 1,
    paddingTop: spacing.md,
    paddingBottom: spacing.xl + spacing.md,
    paddingHorizontal: spacing.sm,
  },
  option: { paddingHorizontal: spacing.md, paddingVertical: spacing.sm + 4, borderRadius: radius.xl },
})
