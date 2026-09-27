import { useState } from 'react'
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, TextInput, View } from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'
import { useTranslation } from 'react-i18next'

import { useAuth } from '@/auth/AuthProvider'
import { Button } from '@/components/ui/Button'
import { GlassCard } from '@/components/ui/GlassCard'
import { Body, Heading, Muted } from '@/components/ui/Text'
import { fonts, radius, spacing, useTheme } from '@/theme'

export default function LoginScreen() {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const { signIn, serverUrl } = useAuth()
  const insets = useSafeAreaInsets()

  // Adresa serveru se předvyplní z minula: po odhlášení se mění heslo, ne server.
  const [url, setUrl] = useState(serverUrl || 'https://')
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      await signIn({ serverUrl: url, login, password })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('mobile.login.failed'))
    } finally {
      setBusy(false)
    }
  }

  const input = [styles.input, { backgroundColor: colors.card, borderColor: colors.border, color: colors.foreground }]

  return (
    <KeyboardAvoidingView style={{ flex: 1, backgroundColor: colors.background }} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerStyle={[styles.content, { paddingTop: insets.top + spacing.xl }]} keyboardShouldPersistTaps="handled">
        <GlassCard glow>
          <Heading size={34}>Libriter</Heading>
          <Muted size={15} style={{ marginTop: spacing.xs, marginBottom: spacing.lg }}>
            {t('mobile.login.subtitle')}
          </Muted>

          <Field label={t('mobile.login.serverUrl')}>
            <TextInput
              style={input}
              value={url}
              onChangeText={setUrl}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="url"
              placeholder={t('mobile.login.serverUrlPlaceholder')}
              placeholderTextColor={colors.mutedForeground}
            />
          </Field>
          <Field label={t('mobile.login.login')}>
            <TextInput
              style={input}
              value={login}
              onChangeText={setLogin}
              autoCapitalize="none"
              autoCorrect={false}
              textContentType="username"
              autoComplete="username"
            />
          </Field>
          <Field label={t('mobile.login.password')}>
            <TextInput style={input} value={password} onChangeText={setPassword} secureTextEntry textContentType="password" onSubmitEditing={() => void submit()} />
          </Field>

          {error !== '' ? (
            <Body size={14} style={{ color: colors.destructive, marginBottom: spacing.sm }}>
              {error}
            </Body>
          ) : null}

          <Button size="lg" label={t('mobile.login.submit')} onPress={() => void submit()} loading={busy} />
        </GlassCard>

        <Muted size={13} style={{ textAlign: 'center', marginTop: spacing.lg }}>
          {t('mobile.login.footer')}
        </Muted>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <View style={{ gap: spacing.xs, marginBottom: spacing.md }}>
      <Muted size={13}>{label}</Muted>
      {children}
    </View>
  )
}

const styles = StyleSheet.create({
  content: { padding: spacing.md },
  input: { borderWidth: 1, borderRadius: radius.lg, fontFamily: fonts.sans, fontSize: 16, paddingHorizontal: spacing.md, paddingVertical: spacing.sm + 4 },
})
