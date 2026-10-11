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
  type UpdateUserRequest,
  type User,
} from 'core-shared'

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

export function useChangePassword(userId: string) {
  return useMutation({
    mutationFn: (body: ChangePasswordRequest) =>
      apiFetch<{ status: string }>(`/users/${userId}/password`, { method: 'PUT', json: body }),
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
