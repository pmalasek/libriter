import { useCallback, useEffect, useState } from 'react'
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Switch, TextInput, View } from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'
import { useTranslation } from 'react-i18next'
import { Eye, EyeOff, FingerprintPattern } from 'lucide-react-native'

import { useAuth } from '@/auth/AuthProvider'
import { biometricsAvailable, clearCredentials, hasSavedCredentials, loadCredentials, saveCredentials } from '@/auth/credentials'
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
  const [showPassword, setShowPassword] = useState(false)
  const [canRemember] = useState(biometricsAvailable)
  const [remember, setRemember] = useState(false)
  const [hasSaved, setHasSaved] = useState(false)

  const fillSaved = useCallback(async () => {
    const saved = await loadCredentials()
    if (!saved) return
    setLogin(saved.login)
    setPassword(saved.password)
  }, [])

  // Uložené údaje se jen předvyplní, přihlásit se musí uživatel sám: po
  // odhlášení nemá aplikace hned zase naskočit.
  useEffect(() => {
    if (!canRemember) return
    let cancelled = false
    void hasSavedCredentials().then((saved) => {
      if (cancelled || !saved) return
      setHasSaved(true)
      setRemember(true)
      void fillSaved()
    })
    return () => {
      cancelled = true
    }
  }, [canRemember, fillSaved])

  const forgetSaved = async () => {
    await clearCredentials()
    setHasSaved(false)
    setRemember(false)
  }

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      await signIn({ serverUrl: url, login, password })
      // Přihlášení už proběhlo; nevydařené uložení (zrušený biometrický
      // dotaz na Androidu) ho nesmí zvrátit.
      try {
        if (canRemember && remember) await saveCredentials({ login: login.trim(), password })
        else if (hasSaved) await clearCredentials()
      } catch {
        // Příště se jen nic nepředvyplní.
      }
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
            <View>
              <TextInput
                style={[input, styles.passwordInput]}
                value={password}
                onChangeText={setPassword}
                secureTextEntry={!showPassword}
                autoCapitalize="none"
                autoCorrect={false}
                textContentType="password"
                autoComplete="password"
                onSubmitEditing={() => void submit()}
              />
              <Pressable
                style={styles.eye}
                onPress={() => setShowPassword((v) => !v)}
                hitSlop={8}
                accessibilityRole="button"
                accessibilityLabel={t(showPassword ? 'mobile.login.hidePassword' : 'mobile.login.showPassword')}
              >
                {showPassword ? <EyeOff size={20} color={colors.mutedForeground} /> : <Eye size={20} color={colors.mutedForeground} />}
              </Pressable>
            </View>
          </Field>

          {canRemember ? (
            <View style={styles.rememberRow}>
              <View style={{ flex: 1 }}>
                <Body size={14} medium>
                  {t('mobile.login.remember')}
                </Body>
                <Muted size={12}>{t('mobile.login.rememberHint')}</Muted>
              </View>
              <Switch value={remember} onValueChange={setRemember} trackColor={{ true: colors.primary, false: colors.border }} />
            </View>
          ) : null}

          {error !== '' ? (
            <Body size={14} style={{ color: colors.destructive, marginBottom: spacing.sm }}>
              {error}
            </Body>
          ) : null}

          <Button size="lg" label={t('mobile.login.submit')} onPress={() => void submit()} loading={busy} />

          {canRemember && hasSaved ? (
            <View style={styles.savedActions}>
              <Pressable style={styles.savedAction} onPress={() => void fillSaved()} hitSlop={8} accessibilityRole="button">
                <FingerprintPattern size={16} color={colors.primary} />
                <Body size={14} style={{ color: colors.primary }}>
                  {t('mobile.login.fillSaved')}
                </Body>
              </Pressable>
              <Pressable onPress={() => void forgetSaved()} hitSlop={8} accessibilityRole="button">
                <Muted size={13}>{t('mobile.login.forgetSaved')}</Muted>
              </Pressable>
            </View>
          ) : null}
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
  passwordInput: { paddingRight: spacing.md + 32 },
  eye: { position: 'absolute', right: spacing.md, top: 0, bottom: 0, justifyContent: 'center' },
  rememberRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md, marginBottom: spacing.md },
  savedActions: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: spacing.sm, marginTop: spacing.md },
  savedAction: { flexDirection: 'row', alignItems: 'center', gap: spacing.xs },
})
