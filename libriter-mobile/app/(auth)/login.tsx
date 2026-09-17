import { useState } from 'react'
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { useAuth } from '@/auth/AuthProvider'
import { colors, radius, spacing } from '@/theme'

export default function LoginScreen() {
  const { signIn, serverUrl } = useAuth()
  const insets = useSafeAreaInsets()

  // Adresa serveru se předvyplní z minula: po odhlášení se mění heslo,
  // ne server.
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

  return (
    <KeyboardAvoidingView
      style={styles.flex}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
    >
      <ScrollView
        contentContainerStyle={[styles.content, { paddingTop: insets.top + spacing.xl }]}
        keyboardShouldPersistTaps="handled"
      >
        <Text style={styles.title}>Libriter</Text>
        <Text style={styles.subtitle}>Přihlaste se ke svému serveru s audioknihami.</Text>

        <Field label="Adresa serveru">
          <TextInput
            style={styles.input}
            value={url}
            onChangeText={setUrl}
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="url"
            placeholder="https://libriter.doma.cz"
            placeholderTextColor={colors.textMuted}
          />
        </Field>

        <Field label="E-mail">
          <TextInput
            style={styles.input}
            value={email}
            onChangeText={setEmail}
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="email-address"
            textContentType="username"
            placeholderTextColor={colors.textMuted}
          />
        </Field>

        <Field label="Heslo">
          <TextInput
            style={styles.input}
            value={password}
            onChangeText={setPassword}
            secureTextEntry
            textContentType="password"
            onSubmitEditing={() => void submit()}
          />
        </Field>

        {error !== '' && <Text style={styles.error}>{error}</Text>}

        <Pressable
          style={({ pressed }) => [styles.button, (pressed || busy) && styles.buttonPressed]}
          onPress={() => void submit()}
          disabled={busy}
        >
          {busy ? (
            <ActivityIndicator color={colors.accentText} />
          ) : (
            <Text style={styles.buttonText}>Přihlásit se</Text>
          )}
        </Pressable>

        <Text style={styles.note}>
          Aplikace je jen přehrávač. Knihovnu spravujte ve webovém rozhraní.
        </Text>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      {children}
    </View>
  )
}

const styles = StyleSheet.create({
  flex: { flex: 1, backgroundColor: colors.background },
  content: { padding: spacing.lg, gap: spacing.md },
  title: { color: colors.text, fontSize: 34, fontWeight: '700' },
  subtitle: { color: colors.textMuted, fontSize: 15, marginBottom: spacing.md },
  field: { gap: spacing.xs },
  label: { color: colors.textMuted, fontSize: 13 },
  input: {
    backgroundColor: colors.surface,
    borderColor: colors.border,
    borderWidth: 1,
    borderRadius: radius.sm,
    color: colors.text,
    fontSize: 16,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm + 4,
  },
  error: { color: colors.danger, fontSize: 14 },
  button: {
    backgroundColor: colors.accent,
    borderRadius: radius.sm,
    paddingVertical: spacing.md,
    alignItems: 'center',
    marginTop: spacing.sm,
  },
  buttonPressed: { opacity: 0.7 },
  buttonText: { color: colors.accentText, fontSize: 16, fontWeight: '600' },
  note: { color: colors.textMuted, fontSize: 13, textAlign: 'center', marginTop: spacing.md },
})
