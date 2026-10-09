import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  apiFetch,
  coreQueryKeys as queryKeys,
  type AuthConfig,
  type AuthResponse,
  type ChangePasswordRequest,
  type LoginRequest,
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
