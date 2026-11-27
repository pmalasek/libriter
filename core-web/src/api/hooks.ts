import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  apiFetch,
  coreQueryKeys as queryKeys,
  type AuthConfig,
  type AuthResponse,
  type ChangePasswordRequest,
  type DeviceInfo,
  type LoginRequest,
  type PluginInfo,
  type PluginsResponse,
  type RegisterRequest,
  type DeviceSession,
  type SessionResetResponse,
  type UpdateUserRequest,
  type User,
} from 'core-shared'
import { useAuth } from '../auth/AuthContext'

// Hooky účtu společné všem produktům: přihlášení, registrace a profil.

export function useUser(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.user(id ?? ''),
    queryFn: ({ signal }) => apiFetch<User>(`/users/${id}`, { signal }),
    enabled: Boolean(id),
    refetchOnWindowFocus: true,
  })
}

/**
 * Veřejné nastavení přihlášení – je registrace zapnutá a jakou roli nový účet
 * dostane. Mění se zřídka, takže stačí načíst jednou za relaci.
 */
export function useAuthConfig() {
  return useQuery({
    queryKey: queryKeys.authConfig,
    queryFn: () => apiFetch<AuthConfig>('/auth/config', { anonymous: true }),
    staleTime: 5 * 60 * 1000,
  })
}

export function useLogin() {
  return useMutation({
    mutationFn: (body: LoginRequest) =>
      apiFetch<AuthResponse>('/auth/login', { method: 'POST', json: body, anonymous: true }),
  })
}

export function useRegister() {
  return useMutation({
    mutationFn: (body: RegisterRequest) =>
      apiFetch<AuthResponse>('/auth/register', { method: 'POST', json: body, anonymous: true }),
  })
}

export function useUpdateProfile(userId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: UpdateUserRequest) =>
      apiFetch<User>(`/users/${userId}`, { method: 'PUT', json: body }),
    onSuccess: (user) => {
      queryClient.setQueryData(queryKeys.user(userId), user)
    },
  })
}

/**
 * Změna hesla. Server při ní odhlásí všechna přihlášení; u vlastního účtu
 * vrátí nový token, který se hned uloží, aby web zůstal přihlášený.
 */
export function useChangePassword(userId: string) {
  const queryClient = useQueryClient()
  const { replaceToken } = useAuth()
  return useMutation({
    mutationFn: (body: ChangePasswordRequest) =>
      apiFetch<SessionResetResponse>(`/users/${userId}/password`, { method: 'PUT', json: body }),
    onSuccess: (res) => {
      if (res.token) replaceToken(res.token)
      void queryClient.invalidateQueries({ queryKey: queryKeys.userDevices(userId) })
    },
  })
}

// ---- pluginy metadat (admin, společné všem produktům) ----

export function usePlugins() {
  return useQuery({
    queryKey: queryKeys.adminPlugins,
    queryFn: () => apiFetch<PluginsResponse>('/admin/plugins'),
  })
}

export function useReloadPlugins() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch<PluginsResponse>('/admin/plugins/reload', { method: 'POST' }),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKeys.adminPlugins, data)
      // Seznamy zdrojů v produktech (výběr zdroje u knihovny, pořadí zdrojů).
      void queryClient.invalidateQueries()
    },
  })
}

/** Uloží konfiguraci pluginu; null = beze změny, '' = smazat. */
export function useSetPluginConfig(name: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (values: Record<string, string | null>) =>
      apiFetch<PluginInfo>(`/admin/plugins/${name}/config`, { method: 'PUT', json: values }),
    onSuccess: (info) => {
      queryClient.setQueryData<PluginsResponse>(queryKeys.adminPlugins, (prev) =>
        prev ? { ...prev, plugins: prev.plugins.map((p) => (p.name === info.name ? info : p)) } : prev,
      )
    },
  })
}

/** Čekající párování televize podle kódu z obrazovky (stránka /link). */
export function useDeviceInfo(code: string) {
  return useQuery({
    queryKey: ['device', code],
    queryFn: ({ signal }) => apiFetch<DeviceInfo>(`/auth/device/${encodeURIComponent(code)}`, { signal }),
    enabled: code.length > 0,
    retry: false,
  })
}

export function useApproveDevice() {
  return useMutation({
    mutationFn: (userCode: string) =>
      apiFetch<DeviceInfo>('/auth/device/approve', { method: 'POST', json: { user_code: userCode } }),
  })
}

// ---- přihlášená zařízení a odhlášení všude (profil) ----

/** Přihlášená zařízení uživatele (mobilní aplikace, televize). */
export function useUserDevices(userId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.userDevices(userId ?? ''),
    queryFn: ({ signal }) => apiFetch<DeviceSession[]>(`/users/${userId}/devices`, { signal }),
    enabled: Boolean(userId),
  })
}

/** Odhlásí jedno zařízení; ostatní zůstanou přihlášená. */
export function useRevokeDevice(userId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (sessionId: string) =>
      apiFetch<void>(`/users/${userId}/devices/${sessionId}`, { method: 'DELETE' }),
    onSuccess: (_, sessionId) => {
      queryClient.setQueryData<DeviceSession[]>(queryKeys.userDevices(userId), (prev) =>
        prev?.filter((d) => d.id !== sessionId),
      )
    },
  })
}

/**
 * Odhlásí účet všude. U vlastního účtu server vrátí nový token, který se
 * hned uloží – tento prohlížeč zůstane přihlášený.
 */
export function useLogoutAll(userId: string) {
  const queryClient = useQueryClient()
  const { replaceToken } = useAuth()
  return useMutation({
    mutationFn: () => apiFetch<SessionResetResponse>(`/users/${userId}/logout-all`, { method: 'POST' }),
    onSuccess: (res) => {
      if (res.token) replaceToken(res.token)
      queryClient.setQueryData<DeviceSession[]>(queryKeys.userDevices(userId), [])
    },
  })
}
