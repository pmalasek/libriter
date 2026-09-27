// Hooky administrace (/api/v1/admin/*). Drží se stranou od hooks.ts, aby
// běžná práce s knihovnou zůstala přehledná.

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from '@tanstack/react-query'
import { apiFetch, asList } from './client'
import { queryKeys } from './hooks'
import type {
  AuditPage,
  CreateUserRequest,
  DuplicateGroup,
  DuplicateReport,
  LibraryStats,
  ListeningDetail,
  ListeningSummary,
  MergePlan,
  MergeResult,
  MetadataSettings,
  MetadataSettingsRequest,
  RegistrationSettings,
  RepairPlan,
  RepairResult,
  Role,
  ScannerStatus,
  SystemInfo,
  UpdateUserRequest,
  User,
} from './types'

export const adminKeys = {
  users: ['admin', 'users'] as const,
  metadata: ['admin', 'settings', 'metadata'] as const,
  registration: ['admin', 'settings', 'registration'] as const,
  scanner: ['admin', 'scanner'] as const,
  duplicates: ['admin', 'library', 'duplicates'] as const,
  stats: ['admin', 'stats'] as const,
  system: ['admin', 'system'] as const,
  audit: ['admin', 'audit'] as const,
  listening: ['admin', 'listening'] as const,
  listeningUser: (userId: string) => ['admin', 'listening', userId] as const,
}

// --- čtení ---

export function useAdminUsers(): UseQueryResult<User[], Error> {
  return useQuery({
    queryKey: adminKeys.users,
    queryFn: async () => asList(await apiFetch<User[] | null>('/users')),
  })
}

export function useMetadataSettings() {
  return useQuery({
    queryKey: adminKeys.metadata,
    queryFn: () => apiFetch<MetadataSettings>('/admin/settings/metadata'),
  })
}

export function useRegistrationSettings() {
  return useQuery({
    queryKey: adminKeys.registration,
    queryFn: () => apiFetch<RegistrationSettings>('/admin/settings/registration'),
  })
}

/** Stav scanneru; během běhu se obnovuje často, jinak jen občas. */
export function useScannerStatus() {
  return useQuery({
    queryKey: adminKeys.scanner,
    queryFn: () => apiFetch<ScannerStatus>('/admin/scanner'),
    refetchInterval: (query) => (query.state.data?.running ? 2000 : 15000),
  })
}

export function useLibraryStats() {
  return useQuery({
    queryKey: adminKeys.stats,
    queryFn: () => apiFetch<LibraryStats>('/admin/stats'),
  })
}

export function useSystemInfo() {
  return useQuery({
    queryKey: adminKeys.system,
    queryFn: () => apiFetch<SystemInfo>('/admin/system'),
  })
}

/** Přehled poslechu všech uživatelů; jen pro čtení, nic se tím nemění. */
export function useListeningOverview(): UseQueryResult<ListeningSummary[], Error> {
  return useQuery({
    queryKey: adminKeys.listening,
    queryFn: async () => asList(await apiFetch<ListeningSummary[] | null>('/admin/listening')),
  })
}

/** Poslechy, stav knih a deník jednoho uživatele. */
export function useListeningUser(userId: string) {
  return useQuery({
    queryKey: adminKeys.listeningUser(userId),
    queryFn: () => apiFetch<ListeningDetail>(`/admin/listening/${userId}`),
    enabled: Boolean(userId),
  })
}

const AUDIT_PAGE_SIZE = 50

/** Audit log se stránkuje kurzorem (id posledního záznamu), ne offsetem. */
export function useAuditLog() {
  return useInfiniteQuery({
    queryKey: adminKeys.audit,
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      apiFetch<AuditPage>(
        `/admin/audit?limit=${AUDIT_PAGE_SIZE}${pageParam ? `&before=${pageParam}` : ''}`,
      ),
    getNextPageParam: (last) => last.next_before ?? undefined,
  })
}

// --- mutace ---

/** Po zásahu admina zestará seznam uživatelů i audit log. */
function useAdminMutation<TVars, TData>(
  mutationFn: (vars: TVars) => Promise<TData>,
  extraKeys: readonly (readonly string[])[] = [],
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: adminKeys.audit })
      for (const key of extraKeys) {
        void queryClient.invalidateQueries({ queryKey: key })
      }
    },
  })
}

export function useCreateUser() {
  return useAdminMutation(
    (body: CreateUserRequest) => apiFetch<User>('/admin/users', { method: 'POST', json: body }),
    [adminKeys.users, adminKeys.stats],
  )
}

/** Úprava údajů libovolného účtu adminem – stejný endpoint jako vlastní profil. */
export function useUpdateUser() {
  return useAdminMutation(
    ({ userId, ...body }: UpdateUserRequest & { userId: string }) =>
      apiFetch<User>(`/users/${userId}`, { method: 'PUT', json: body }),
    [adminKeys.users],
  )
}

export function useSetUserRole() {
  return useAdminMutation(
    ({ userId, role }: { userId: string; role: Role }) =>
      apiFetch<{ status: string; role: Role }>(`/users/${userId}/role`, {
        method: 'PUT',
        json: { role },
      }),
    [adminKeys.users],
  )
}

export function useDeleteUser() {
  return useAdminMutation(
    (userId: string) => apiFetch<void>(`/users/${userId}`, { method: 'DELETE' }),
    // Smazaný účet musí zmizet i z přehledu poslechů; klíč se shoduje
    // prefixem, takže padne i jeho detail.
    [adminKeys.users, adminKeys.stats, adminKeys.listening],
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

export function useSaveMetadataSettings() {
  return useAdminMutation(
    (body: MetadataSettingsRequest) =>
      apiFetch<MetadataSettings>('/admin/settings/metadata', { method: 'PUT', json: body }),
    [adminKeys.metadata],
  )
}

export function useSaveRegistrationSettings() {
  return useAdminMutation(
    (body: RegistrationSettings) =>
      apiFetch<RegistrationSettings>('/admin/settings/registration', { method: 'PUT', json: body }),
    [adminKeys.registration, queryKeys.authConfig],
  )
}

export function useTriggerRescan() {
  return useAdminMutation(
    () => apiFetch<ScannerStatus>('/admin/scanner/rescan', { method: 'POST' }),
    [adminKeys.scanner],
  )
}

/**
 * Náhled opravy kapitol. Je to mutace, ne dotaz: server při něm prochází celý
 * AUDIO_ROOT, takže se má spustit jen na kliknutí, ne na pozadí.
 */
export function usePlanRepair() {
  return useMutation({
    mutationFn: () => apiFetch<RepairPlan>('/admin/library/repair'),
  })
}

/**
 * Provedení opravy. `forceMissing` přebíjí pojistku proti nepřipojenému disku –
 * bez něj se při sepnuté pojistce chybějící soubory přeskočí. Plán si server
 * sestaví znovu sám, klient mu seznam knih ke smazání neposílá.
 */
export function useApplyRepair() {
  return useAdminMutation(
    (forceMissing: boolean = false) =>
      apiFetch<RepairResult>('/admin/library/repair', {
        method: 'POST',
        json: { force_missing: forceMissing },
      }),
    [adminKeys.scanner, adminKeys.stats, queryKeys.books],
  )
}

/**
 * Hlášení duplicitních knih. Na rozdíl od ostatních kontrol je to dotaz, ne
 * mutace: čte se jen databáze (žádný průchod diskem), takže může běžet hned
 * po otevření záložky a rovnou ukázat, co je špatně.
 */
export function useDuplicateReport(): UseQueryResult<DuplicateReport, Error> {
  return useQuery({
    queryKey: adminKeys.duplicates,
    queryFn: () => apiFetch<DuplicateReport>('/admin/library/duplicates'),
  })
}

/**
 * „Není to duplicita.“ Skupina zmizí z nálezů, ale zůstane mezi odmítnutými,
 * takže rozhodnutí jde vzít zpět.
 */
export function useDismissDuplicate() {
  return useAdminMutation(
    (key: string) =>
      apiFetch<DuplicateGroup>('/admin/library/duplicates/dismiss', {
        method: 'POST',
        json: { key },
      }),
    [adminKeys.duplicates, adminKeys.stats],
  )
}

/** Vrátí odmítnutou skupinu zpátky mezi nálezy. */
export function useRestoreDuplicate() {
  return useAdminMutation(
    (key: string) =>
      apiFetch<void>(`/admin/library/duplicates/dismiss?key=${encodeURIComponent(key)}`, {
        method: 'DELETE',
      }),
    [adminKeys.duplicates, adminKeys.stats],
  )
}

/** Náhled sloučení rozdělených knih – mutace ze stejného důvodu jako usePlanRepair. */
export function usePlanMerge() {
  return useMutation({
    mutationFn: () => apiFetch<MergePlan>('/admin/library/merge'),
  })
}

/**
 * Sloučení vybraných skupin; posílají se jen ID cílových knih, plán si server
 * sestaví znovu sám. Klíč ['books'] zneplatní i detaily jednotlivých knih
 * (['books', id]) – TanStack porovnává klíče podle prefixu.
 */
export function useApplyMerge() {
  return useAdminMutation(
    (targets: string[]) =>
      apiFetch<MergeResult>('/admin/library/merge', { method: 'POST', json: { targets } }),
    [adminKeys.stats, queryKeys.books],
  )
}
