import { lazy } from 'react'
import { Route, Routes } from 'react-router'
import { RedirectIfAuthenticated, RequireAuth } from 'core-web/src/auth/RequireAuth'
import { RequireRole } from 'core-web/src/auth/RequireRole'
import { LoginPage } from 'core-web/src/pages/LoginPage'
import { RegisterPage } from 'core-web/src/pages/RegisterPage'
import { DisabledGate } from '@/components/DisabledGate'
import { AppLayout } from '@/components/layout/AppLayout'

const lazyPage = <K extends string>(load: () => Promise<Record<K, React.ComponentType>>, name: K) =>
  lazy(() => load().then((m) => ({ default: m[name] })))

const HomePage = lazyPage(() => import('@/pages/HomePage'), 'HomePage')
const MoviesPage = lazyPage(() => import('@/pages/ListPage'), 'MoviesPage')
const ShowsPage = lazyPage(() => import('@/pages/ListPage'), 'ShowsPage')
const LibraryPage = lazyPage(() => import('@/pages/ListPage'), 'LibraryPage')
const MovieDetailPage = lazyPage(() => import('@/pages/MovieDetailPage'), 'MovieDetailPage')
const ShowDetailPage = lazyPage(() => import('@/pages/ShowDetailPage'), 'ShowDetailPage')
const ShowFilesPage = lazyPage(() => import('@/pages/ShowFilesPage'), 'ShowFilesPage')
const WatchPage = lazyPage(() => import('@/pages/WatchPage'), 'WatchPage')
const WatchlistPage = lazyPage(() => import('@/pages/WatchlistPage'), 'WatchlistPage')
const SearchPage = lazyPage(() => import('@/pages/SearchPage'), 'SearchPage')
const PersonPage = lazyPage(() => import('@/pages/PersonPage'), 'PersonPage')
const NotFoundPage = lazyPage(() => import('@/pages/NotFoundPage'), 'NotFoundPage')
const ProfilePage = lazyPage(() => import('core-web/src/pages/ProfilePage'), 'ProfilePage')
const LinkDevicePage = lazyPage(() => import('core-web/src/pages/LinkDevicePage'), 'LinkDevicePage')
const AdminLayout = lazyPage(() => import('@/pages/admin/AdminLayout'), 'AdminLayout')
const AdminOverviewPage = lazyPage(() => import('@/pages/admin/AdminOverviewPage'), 'AdminOverviewPage')
const AdminLibrariesPage = lazyPage(() => import('@/pages/admin/AdminLibrariesPage'), 'AdminLibrariesPage')
const AdminSourcesPage = lazyPage(() => import('@/pages/admin/AdminSourcesPage'), 'AdminSourcesPage')
const AdminMatchPage = lazyPage(() => import('@/pages/admin/AdminMatchPage'), 'AdminMatchPage')
const AdminIssuesPage = lazyPage(() => import('@/pages/admin/AdminIssuesPage'), 'AdminIssuesPage')
const AdminTranscodePage = lazyPage(() => import('@/pages/admin/AdminTranscodePage'), 'AdminTranscodePage')
const AdminTrashPage = lazyPage(() => import('@/pages/admin/AdminTrashPage'), 'AdminTrashPage')
const AdminWatchingPage = lazyPage(() => import('@/pages/admin/AdminWatchingPage'), 'AdminWatchingPage')
const AdminWatchingUserPage = lazyPage(() => import('@/pages/admin/AdminWatchingUserPage'), 'AdminWatchingUserPage')
const AdminUsersPage = lazyPage(() => import('@/pages/admin/AdminUsersPage'), 'AdminUsersPage')
const AdminSettingsPage = lazyPage(() => import('core-web/src/pages/admin/AdminSettingsPage'), 'AdminSettingsPage')
const AdminAuditPage = lazyPage(() => import('core-web/src/pages/admin/AdminAuditPage'), 'AdminAuditPage')

export function App() {
  return (
    <Routes>
      <Route element={<RedirectIfAuthenticated />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
      </Route>

      <Route element={<RequireAuth />}>
        {/* Schválení párování televize – mimo layout, chodí se sem i z telefonu přes QR */}
        <Route path="link" element={<LinkDevicePage />} />
        {/* Přehrávač je přes celou obrazovku, bez navigace. */}
        <Route element={<DisabledGate />}>
          <Route path="watch/:kind/:id" element={<WatchPage />} />
        </Route>

        <Route element={<AppLayout />}>
          <Route path="profile" element={<ProfilePage />} />

          <Route element={<RequireRole role="admin" />}>
            <Route path="admin" element={<AdminLayout />}>
              {/* Účty, registrace a audit patří jádru – fungují i bez VIDEO_ROOT. */}
              <Route path="users" element={<AdminUsersPage />} />
              <Route path="settings" element={<AdminSettingsPage />} />
              <Route path="audit" element={<AdminAuditPage />} />
              <Route element={<DisabledGate />}>
                <Route index element={<AdminOverviewPage />} />
                <Route path="libraries" element={<AdminLibrariesPage />} />
                <Route path="sources" element={<AdminSourcesPage />} />
                <Route path="match" element={<AdminMatchPage />} />
                <Route path="issues" element={<AdminIssuesPage />} />
                <Route path="trash" element={<AdminTrashPage />} />
                <Route path="transcode" element={<AdminTranscodePage />} />
                <Route path="watching" element={<AdminWatchingPage />} />
                <Route path="watching/:userId" element={<AdminWatchingUserPage />} />
              </Route>
            </Route>
          </Route>

          <Route element={<DisabledGate />}>
            <Route index element={<HomePage />} />
            <Route path="movies" element={<MoviesPage />} />
            <Route path="movies/:id" element={<MovieDetailPage />} />
            <Route path="shows" element={<ShowsPage />} />
            <Route path="shows/:id" element={<ShowDetailPage />} />
            <Route element={<RequireRole role="editor" />}>
              <Route path="shows/:id/files" element={<ShowFilesPage />} />
            </Route>
            <Route path="library/:id" element={<LibraryPage />} />
            <Route path="people/:id" element={<PersonPage />} />
            <Route path="watchlist" element={<WatchlistPage />} />
            <Route path="search" element={<SearchPage />} />
          </Route>
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
