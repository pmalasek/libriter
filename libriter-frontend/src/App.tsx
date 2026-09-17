import { lazy } from 'react'
import { Route, Routes } from 'react-router'
import { RedirectIfAuthenticated, RequireAuth } from '@/auth/RequireAuth'
import { RequireRole } from '@/auth/RequireRole'
import { AppLayout } from '@/components/layout/AppLayout'
import { LoginPage } from '@/pages/LoginPage'
import { RegisterPage } from '@/pages/RegisterPage'

// Kód stránek knihovny načítáme až při jejich otevření.
const AuthorDetailPage = lazy(() => import('@/pages/AuthorDetailPage').then((module) => ({ default: module.AuthorDetailPage })))
const AuthorsPage = lazy(() => import('@/pages/AuthorsPage').then((module) => ({ default: module.AuthorsPage })))
const BookDetailPage = lazy(() => import('@/pages/BookDetailPage').then((module) => ({ default: module.BookDetailPage })))
const BooksPage = lazy(() => import('@/pages/BooksPage').then((module) => ({ default: module.BooksPage })))
const HomePage = lazy(() => import('@/pages/HomePage').then((module) => ({ default: module.HomePage })))
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage').then((module) => ({ default: module.NotFoundPage })))
const ProfilePage = lazy(() => import('@/pages/ProfilePage').then((module) => ({ default: module.ProfilePage })))
const SeriesDetailPage = lazy(() => import('@/pages/SeriesDetailPage').then((module) => ({ default: module.SeriesDetailPage })))
const SeriesPage = lazy(() => import('@/pages/SeriesPage').then((module) => ({ default: module.SeriesPage })))
const SessionsPage = lazy(() => import('@/pages/SessionsPage').then((module) => ({ default: module.SessionsPage })))

// Administrace – kód se stáhne, až když ji admin otevře.
const AdminLayout = lazy(() => import('@/pages/admin/AdminLayout').then((module) => ({ default: module.AdminLayout })))
const AdminOverviewPage = lazy(() => import('@/pages/admin/AdminOverviewPage').then((module) => ({ default: module.AdminOverviewPage })))
const AdminUsersPage = lazy(() => import('@/pages/admin/AdminUsersPage').then((module) => ({ default: module.AdminUsersPage })))
const AdminMetadataPage = lazy(() => import('@/pages/admin/AdminMetadataPage').then((module) => ({ default: module.AdminMetadataPage })))
const AdminLibraryPage = lazy(() => import('@/pages/admin/AdminLibraryPage').then((module) => ({ default: module.AdminLibraryPage })))
const AdminSettingsPage = lazy(() => import('@/pages/admin/AdminSettingsPage').then((module) => ({ default: module.AdminSettingsPage })))
const AdminAuditPage = lazy(() => import('@/pages/admin/AdminAuditPage').then((module) => ({ default: module.AdminAuditPage })))
const AdminListeningPage = lazy(() => import('@/pages/admin/AdminListeningPage').then((module) => ({ default: module.AdminListeningPage })))
const AdminListeningUserPage = lazy(() => import('@/pages/admin/AdminListeningUserPage').then((module) => ({ default: module.AdminListeningUserPage })))

export function App() {
  return (
    <Routes>
      <Route element={<RedirectIfAuthenticated />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
      </Route>

      <Route element={<RequireAuth />}>
        <Route element={<AppLayout />}>
          {/* Domovská stránka: čím se dá pokračovat a co je v knihovně nového. */}
          <Route index element={<HomePage />} />
          <Route path="books" element={<BooksPage />} />
          <Route path="books/:id" element={<BookDetailPage />} />
          <Route path="authors" element={<AuthorsPage />} />
          <Route path="authors/:id" element={<AuthorDetailPage />} />
          <Route path="series" element={<SeriesPage />} />
          <Route path="series/:id" element={<SeriesDetailPage />} />
          <Route path="sessions" element={<SessionsPage />} />
          <Route path="profile" element={<ProfilePage />} />

          <Route element={<RequireRole role="admin" />}>
            <Route path="admin" element={<AdminLayout />}>
              <Route index element={<AdminOverviewPage />} />
              <Route path="users" element={<AdminUsersPage />} />
              <Route path="listening" element={<AdminListeningPage />} />
              <Route path="listening/:userId" element={<AdminListeningUserPage />} />
              <Route path="metadata" element={<AdminMetadataPage />} />
              <Route path="library" element={<AdminLibraryPage />} />
              <Route path="settings" element={<AdminSettingsPage />} />
              <Route path="audit" element={<AdminAuditPage />} />
            </Route>
          </Route>

          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
