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
const WatchPage = lazyPage(() => import('@/pages/WatchPage'), 'WatchPage')
const WatchlistPage = lazyPage(() => import('@/pages/WatchlistPage'), 'WatchlistPage')
const SearchPage = lazyPage(() => import('@/pages/SearchPage'), 'SearchPage')
const NotFoundPage = lazyPage(() => import('@/pages/NotFoundPage'), 'NotFoundPage')
const ProfilePage = lazyPage(() => import('core-web/src/pages/ProfilePage'), 'ProfilePage')
const AdminLayout = lazyPage(() => import('@/pages/admin/AdminLayout'), 'AdminLayout')
const AdminOverviewPage = lazyPage(() => import('@/pages/admin/AdminOverviewPage'), 'AdminOverviewPage')
const AdminLibrariesPage = lazyPage(() => import('@/pages/admin/AdminLibrariesPage'), 'AdminLibrariesPage')
const AdminSourcesPage = lazyPage(() => import('@/pages/admin/AdminSourcesPage'), 'AdminSourcesPage')
const AdminMatchPage = lazyPage(() => import('@/pages/admin/AdminMatchPage'), 'AdminMatchPage')
const AdminTranscodePage = lazyPage(() => import('@/pages/admin/AdminTranscodePage'), 'AdminTranscodePage')

export function App() {
  return (
    <Routes>
      <Route element={<RedirectIfAuthenticated />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
      </Route>

      <Route element={<RequireAuth />}>
        {/* Přehrávač je přes celou obrazovku, bez navigace. */}
        <Route element={<DisabledGate />}>
          <Route path="watch/:kind/:id" element={<WatchPage />} />
        </Route>

        <Route element={<AppLayout />}>
          <Route path="profile" element={<ProfilePage />} />
          <Route element={<DisabledGate />}>
            <Route index element={<HomePage />} />
            <Route path="movies" element={<MoviesPage />} />
            <Route path="movies/:id" element={<MovieDetailPage />} />
            <Route path="shows" element={<ShowsPage />} />
            <Route path="shows/:id" element={<ShowDetailPage />} />
            <Route path="library/:id" element={<LibraryPage />} />
            <Route path="watchlist" element={<WatchlistPage />} />
            <Route path="search" element={<SearchPage />} />

            <Route element={<RequireRole role="admin" />}>
              <Route path="admin" element={<AdminLayout />}>
                <Route index element={<AdminOverviewPage />} />
                <Route path="libraries" element={<AdminLibrariesPage />} />
                <Route path="sources" element={<AdminSourcesPage />} />
                <Route path="match" element={<AdminMatchPage />} />
                <Route path="transcode" element={<AdminTranscodePage />} />
              </Route>
            </Route>
          </Route>
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
