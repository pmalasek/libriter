import { StyleSheet, TextInput, View } from 'react-native'
import { Search } from 'lucide-react-native'

import { fonts, radius, spacing, useTheme } from '@/theme'

/** Vyhledávací pole v hlavičce seznamů – jako `Input type="search"` na webu. */
export function SearchInput({
  value,
  onChangeText,
  placeholder,
}: {
  value: string
  onChangeText: (value: string) => void
  placeholder: string
}) {
  const { colors } = useTheme()
  return (
    <View style={[styles.wrap, { backgroundColor: colors.card, borderColor: colors.border }]}>
      <Search color={colors.mutedForeground} size={16} />
      <TextInput
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
        placeholderTextColor={colors.mutedForeground}
        autoCorrect={false}
        autoCapitalize="none"
        clearButtonMode="while-editing"
        returnKeyType="search"
        style={[styles.input, { color: colors.foreground }]}
      />
    </View>
  )
}

const styles = StyleSheet.create({
  wrap: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderWidth: 1,
    borderRadius: radius.lg,
    paddingHorizontal: spacing.md - 4,
    height: 40,
  },
  input: { flex: 1, fontFamily: fonts.sans, fontSize: 15, paddingVertical: 0 },
})
