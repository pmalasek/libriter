import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryKey,
  type UseQueryResult,
} from '@tanstack/react-query'
import {
  apiFetch,
  asList,
  coreQueryKeys as queryKeys,
  type AuditPage,
  type AuthConfig,
  type AuthResponse,
  type ChangePasswordRequest,
  type CreateUserRequest,
  type DeviceInfo,
  type LoginRequest,
  type PluginInfo,
  type PluginsResponse,
  type RegisterRequest,
  type RegistrationSettings,
  type Role,
  type DeviceSession,
  type SessionResetResponse,
  type UpdateUserRequest,
  type User,
} from 'core-shared'
import { useAuth } from '../auth/AuthContext'

// Hooky účtu společné všem produktům: přihlášení, registrace a profil.

/** Verze serveru (patička navigace). Za běhu se nemění, načte se jednou. */
export function useServerVersion() {
  const { user } = useAuth()
  return useQuery({
    queryKey: queryKeys.version,
    queryFn: () => apiFetch<{ version: string }>('/version'),
    enabled: !!user,
    staleTime: Infinity,
    select: (data) => data.version,
  })
}

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

// ---- správa uživatelů, registrace a audit (admin, společné všem produktům) ----

export function useAdminUsers(): UseQueryResult<User[], Error> {
  return useQuery({
    queryKey: queryKeys.adminUsers,
    queryFn: async () => asList(await apiFetch<User[] | null>('/users')),
  })
}

export function useRegistrationSettings() {
  return useQuery({
    queryKey: queryKeys.adminRegistration,
    queryFn: () => apiFetch<RegistrationSettings>('/admin/settings/registration'),
  })
}

const AUDIT_PAGE_SIZE = 50

/** Audit log se stránkuje kurzorem (id posledního záznamu), ne offsetem. */
export function useAuditLog() {
  return useInfiniteQuery({
    queryKey: queryKeys.adminAudit,
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      apiFetch<AuditPage>(
        `/admin/audit?limit=${AUDIT_PAGE_SIZE}${pageParam ? `&before=${pageParam}` : ''}`,
      ),
    getNextPageParam: (last) => last.next_before ?? undefined,
  })
}

/**
 * Klíče produktu, které ukazují účty (statistiky, přehled poslechu či
 * sledování). Jádro je nezná, předává je stránka produktu a mutace účtů je
 * po úspěchu zestarají spolu s klíči jádra.
 */
export type ProductKeys = readonly QueryKey[]

/** Po zásahu admina zestará audit log a zadané klíče. */
function useAdminMutation<TVars, TData>(
  mutationFn: (vars: TVars) => Promise<TData>,
  extraKeys: readonly QueryKey[] = [],
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.adminAudit })
      for (const key of extraKeys) {
        void queryClient.invalidateQueries({ queryKey: key })
      }
    },
  })
}

export function useCreateUser(productKeys: ProductKeys = []) {
  return useAdminMutation(
    (body: CreateUserRequest) => apiFetch<User>('/admin/users', { method: 'POST', json: body }),
    [queryKeys.adminUsers, ...productKeys],
  )
}

/** Úprava údajů libovolného účtu adminem – stejný endpoint jako vlastní profil. */
export function useUpdateUser(productKeys: ProductKeys = []) {
  return useAdminMutation(
    ({ userId, ...body }: UpdateUserRequest & { userId: string }) =>
      apiFetch<User>(`/users/${userId}`, { method: 'PUT', json: body }),
    [queryKeys.adminUsers, ...productKeys],
  )
}

export function useSetUserRole(productKeys: ProductKeys = []) {
  return useAdminMutation(
    ({ userId, role }: { userId: string; role: Role }) =>
      apiFetch<{ status: string; role: Role }>(`/users/${userId}/role`, {
        method: 'PUT',
        json: { role },
      }),
    [queryKeys.adminUsers, ...productKeys],
  )
}

/** Smazaný účet musí zmizet i z přehledů produktu – proto productKeys. */
export function useDeleteUser(productKeys: ProductKeys = []) {
  return useAdminMutation(
    (userId: string) => apiFetch<void>(`/users/${userId}`, { method: 'DELETE' }),
    [queryKeys.adminUsers, ...productKeys],
  )
}

/** Reset hesla adminem jde přes stejný endpoint jako změna vlastního hesla. */
export function useResetUserPassword() {
  return useAdminMutation(({ userId, password }: { userId: string; password: string }) =>
    apiFetch<{ status: string }>(`/users/${userId}/password`, {
      method: 'PUT',
      json: { password },
    }),
  )
}

export function useSaveRegistrationSettings() {
  return useAdminMutation(
    (body: RegistrationSettings) =>
      apiFetch<RegistrationSettings>('/admin/settings/registration', { method: 'PUT', json: body }),
    [queryKeys.adminRegistration, queryKeys.authConfig],
  )
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
