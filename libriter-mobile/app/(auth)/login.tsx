import { useState } from 'react'
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, TextInput, View } from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { useAuth } from '@/auth/AuthProvider'
import { Button } from '@/components/ui/Button'
import { GlassCard } from '@/components/ui/GlassCard'
import { Body, Heading, Muted } from '@/components/ui/Text'
import { fonts, radius, spacing, useTheme } from '@/theme'

export default function LoginScreen() {
  const { colors } = useTheme()
  const { signIn, serverUrl } = useAuth()
  const insets = useSafeAreaInsets()

  // Adresa serveru se předvyplní z minula: po odhlášení se mění heslo, ne server.
  const [url, setUrl] = useState(serverUrl || 'https://')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      await signIn({ serverUrl: url, email, password })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Přihlášení se nepodařilo')
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
            Přihlaste se ke svému serveru s audioknihami.
          </Muted>

          <Field label="Adresa serveru">
            <TextInput
              style={input}
              value={url}
              onChangeText={setUrl}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="url"
              placeholder="https://libriter.doma.cz"
              placeholderTextColor={colors.mutedForeground}
            />
          </Field>
          <Field label="E-mail">
            <TextInput
              style={input}
              value={email}
              onChangeText={setEmail}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="email-address"
              textContentType="username"
            />
          </Field>
          <Field label="Heslo">
            <TextInput style={input} value={password} onChangeText={setPassword} secureTextEntry textContentType="password" onSubmitEditing={() => void submit()} />
          </Field>

          {error !== '' ? (
            <Body size={14} style={{ color: colors.destructive, marginBottom: spacing.sm }}>
              {error}
            </Body>
          ) : null}

          <Button size="lg" label="Přihlásit se" onPress={() => void submit()} loading={busy} />
        </GlassCard>

        <Muted size={13} style={{ textAlign: 'center', marginTop: spacing.lg }}>
          Aplikace je jen přehrávač. Knihovnu spravujte ve webovém rozhraní.
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
