import { useCallback, useEffect, useRef, useState } from 'react'
import { StyleSheet, TextInput, View } from 'react-native'
import { LinearGradient } from 'expo-linear-gradient'
import * as Device from 'expo-device'
import QRCode from 'react-native-qrcode-svg'
import { useTranslation } from 'react-i18next'
import { ArrowRightIcon, RefreshCwIcon, ServerIcon } from 'lucide-react-native'
import { apiFetch, ApiError, type DeviceStart, type DeviceTokenResponse } from 'sceniter-shared'
import { useAuth } from '@/auth/AuthProvider'
import { Button } from '@/components/Button'
import { T } from '@/components/T'
import { normalizeServerUrl } from '@/lib/serverUrl'
import { colors, fonts, px, radius, safe } from '@/theme'

/** Pozadí přihlašovacích obrazovek – tmavé s tyrkysovou září značky. */
function Glow() {
  return (
    <LinearGradient
      colors={['#0E4F4F', '#0D1A1D', colors.bg]}
      locations={[0, 0.5, 1]}
      start={{ x: 0, y: 0 }}
      end={{ x: 0.9, y: 1 }}
      style={StyleSheet.absoluteFill}
    />
  )
}

/** Krok 1: adresa serveru. Ověří se dotazem na /auth/config. */
function ServerStep() {
  const { t } = useTranslation()
  const { server, setServer } = useAuth()
  const [value, setValue] = useState(server ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function connect() {
    const url = normalizeServerUrl(value)
    if (!url) return
    setBusy(true)
    setError(null)
    try {
      // Veřejné nastavení přihlášení má každý server Libriter/Sceniter.
      const res = await fetch(`${url}/api/v1/auth/config`)
      const body: unknown = await res.json()
      if (!res.ok || !body || typeof body !== 'object' || !('registration_enabled' in body)) throw new Error(String(res.status))
      await setServer(url)
    } catch {
      setError(t('video.tv.pair.serverError'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <View style={styles.center}>
      <ServerIcon size={px(72)} color={colors.accent} />
      <T variant="title" style={styles.heading}>
        {t('video.tv.pair.serverTitle')}
      </T>
      <T dim style={styles.hint}>
        {t('video.tv.pair.serverHint')}
      </T>
      <TextInput
        value={value}
        onChangeText={setValue}
        placeholder={t('video.tv.pair.serverPlaceholder')}
        placeholderTextColor={colors.textFaint}
        autoCapitalize="none"
        autoCorrect={false}
        keyboardType="url"
        returnKeyType="go"
        onSubmitEditing={() => void connect()}
        hasTVPreferredFocus
        style={styles.input}
      />
      {error ? <T style={styles.error}>{error}</T> : null}
      <Button
        primary
        icon={ArrowRightIcon}
        label={busy ? t('video.tv.pair.connecting') : t('video.tv.pair.connect')}
        disabled={busy}
        onPress={() => void connect()}
        style={{ marginTop: px(40) }}
      />
    </View>
  )
}

function formatLeft(sec: number): string {
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

/**
 * Krok 2: párovací kód. Televize ukáže kód a QR s odkazem na /link, uživatel
 * ho potvrdí přihlášený na webu a televize si mezitím opakovaně říká o token.
 */
function CodeStep() {
  const { t } = useTranslation()
  const { server, signIn, changeServer } = useAuth()
  const [start, setStart] = useState<DeviceStart | null>(null)
  const [left, setLeft] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)
  const expired = start !== null && left <= 0

  const restart = useCallback(() => {
    setStart(null)
    setError(null)
    setAttempt((a) => a + 1)
  }, [])

  // Nový kód
  useEffect(() => {
    let cancelled = false
    const name = Device.deviceName || Device.modelName || 'TV'
    apiFetch<DeviceStart>('/auth/device/start', { method: 'POST', json: { device_name: name }, anonymous: true })
      .then((s) => {
        if (cancelled) return
        setStart(s)
        setLeft(s.expires_in)
      })
      .catch((e: unknown) => !cancelled && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      cancelled = true
    }
  }, [attempt])

  // Prošlý kód se sám nahradí novým; když server zrovna neodpovídá (restart),
  // zkusí se to znovu za chvíli – uživatel nemusí nic mačkat.
  useEffect(() => {
    if (expired) restart()
  }, [expired, restart])
  useEffect(() => {
    if (!error) return
    const timer = setTimeout(restart, 5000)
    return () => clearTimeout(timer)
  }, [error, restart])

  // Odpočet platnosti
  useEffect(() => {
    if (!start) return
    const timer = setInterval(() => setLeft((l) => Math.max(0, l - 1)), 1000)
    return () => clearInterval(timer)
  }, [start])

  // Dotazování na token
  const signedIn = useRef(false)
  useEffect(() => {
    if (!start || expired) return
    let cancelled = false
    let timer: ReturnType<typeof setTimeout>
    let interval = start.interval * 1000
    const poll = async () => {
      try {
        const res = await apiFetch<DeviceTokenResponse>('/auth/device/token', {
          method: 'POST',
          json: { device_code: start.device_code },
          anonymous: true,
        })
        if (cancelled || signedIn.current) return
        signedIn.current = true
        await signIn({ token: res.token, user: res.user })
        return
      } catch (e) {
        if (cancelled) return
        if (e instanceof ApiError && e.code === 'auth.device_expired') {
          setLeft(0)
          return
        }
        if (e instanceof ApiError && e.code === 'auth.device_slow_down') interval += 2000
        // auth.device_pending i výpadek sítě: zkusit znovu
      }
      timer = setTimeout(() => void poll(), interval)
    }
    timer = setTimeout(() => void poll(), interval)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [expired, signIn, start])

  const host = (server ?? '').replace(/^https?:\/\//, '')
  const link = start ? `${server}${start.verification_path}?code=${encodeURIComponent(start.user_code)}` : ''

  return (
    <View style={styles.pair}>
      <View style={styles.pairText}>
        <T variant="hero">{t('video.tv.pair.title')}</T>
        <T variant="section" dim style={{ marginTop: px(48) }}>
          {t('video.tv.pair.step1')}
        </T>
        <T variant="title" style={{ color: colors.accent, marginTop: px(8) }}>
          {host}/link
        </T>
        <T variant="section" dim style={{ marginTop: px(36) }}>
          {t('video.tv.pair.step2')}
        </T>
        <View style={styles.codeBox}>
          <T style={styles.code}>{start && !expired ? start.user_code : '····-····'}</T>
        </View>
        {error ? (
          <T style={styles.error}>{error}</T>
        ) : expired ? (
          <T style={styles.error}>{t('video.tv.pair.expired')}</T>
        ) : start ? (
          <T dim>
            {t('video.tv.pair.waiting')} · {t('video.tv.pair.expires', { time: formatLeft(left) })}
          </T>
        ) : null}
        <View style={styles.actions}>
          {expired || error ? (
            <Button primary icon={RefreshCwIcon} label={t('video.tv.pair.newCode')} onPress={restart} hasTVPreferredFocus />
          ) : null}
          <Button icon={ServerIcon} label={t('video.tv.pair.changeServer')} onPress={changeServer} hasTVPreferredFocus={!expired && !error} />
        </View>
      </View>
      <View style={styles.qrCol}>
        <View style={styles.qr}>{link && !expired ? <QRCode value={link} size={px(380)} backgroundColor="#FFFFFF" color="#0B0B0F" /> : null}</View>
        <T dim style={{ marginTop: px(28), textAlign: 'center' }}>
          {t('video.tv.pair.scan')}
        </T>
      </View>
    </View>
  )
}

export function PairScreen() {
  const { status } = useAuth()
  return (
    <View style={styles.root}>
      <Glow />
      {status === 'needsServer' ? <ServerStep /> : <CodeStep />}
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: colors.bg },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center', paddingHorizontal: safe.x },
  heading: { marginTop: px(32), textAlign: 'center' },
  hint: { marginTop: px(16), textAlign: 'center', maxWidth: px(1000) },
  input: {
    marginTop: px(56),
    width: px(1000),
    height: px(96),
    borderRadius: radius.button,
    backgroundColor: 'rgba(255,255,255,0.1)',
    color: colors.text,
    paddingHorizontal: px(32),
    fontFamily: fonts.bodyMedium,
    fontSize: px(36),
  },
  error: { color: colors.danger, marginTop: px(20) },
  pair: { flex: 1, flexDirection: 'row', alignItems: 'center', paddingHorizontal: px(160), gap: px(140) },
  pairText: { flex: 1 },
  codeBox: {
    alignSelf: 'flex-start',
    marginTop: px(20),
    marginBottom: px(28),
    paddingHorizontal: px(48),
    paddingVertical: px(20),
    borderRadius: radius.card,
    backgroundColor: 'rgba(255,255,255,0.08)',
    borderWidth: px(2),
    borderColor: 'rgba(255,255,255,0.18)',
  },
  code: { fontFamily: fonts.heading, fontSize: px(110), lineHeight: px(124), letterSpacing: px(10), color: colors.text },
  actions: { flexDirection: 'row', gap: px(24), marginTop: px(56) },
  qrCol: { alignItems: 'center' },
  qr: {
    width: px(440),
    height: px(440),
    borderRadius: px(28),
    backgroundColor: '#FFFFFF',
    alignItems: 'center',
    justifyContent: 'center',
  },
})
