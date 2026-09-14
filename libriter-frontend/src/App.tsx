import { lazy } from 'react'
import { Route, Routes } from 'react-router'
import { RedirectIfAuthenticated, RequireAuth } from '@/auth/RequireAuth'
import { AppLayout } from '@/components/layout/AppLayout'
import { LoginPage } from '@/pages/LoginPage'
import { RegisterPage } from '@/pages/RegisterPage'

// Kód stránek knihovny načítáme až při jejich otevření.
const AuthorDetailPage = lazy(() => import('@/pages/AuthorDetailPage').then((module) => ({ default: module.AuthorDetailPage })))
const AuthorsPage = lazy(() => import('@/pages/AuthorsPage').then((module) => ({ default: module.AuthorsPage })))
const BookDetailPage = lazy(() => import('@/pages/BookDetailPage').then((module) => ({ default: module.BookDetailPage })))
const BooksPage = lazy(() => import('@/pages/BooksPage').then((module) => ({ default: module.BooksPage })))
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage').then((module) => ({ default: module.NotFoundPage })))
const ProfilePage = lazy(() => import('@/pages/ProfilePage').then((module) => ({ default: module.ProfilePage })))
const SeriesDetailPage = lazy(() => import('@/pages/SeriesDetailPage').then((module) => ({ default: module.SeriesDetailPage })))
const SeriesPage = lazy(() => import('@/pages/SeriesPage').then((module) => ({ default: module.SeriesPage })))

export function App() {
  return (
    <Routes>
      <Route element={<RedirectIfAuthenticated />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
      </Route>

      <Route element={<RequireAuth />}>
        <Route element={<AppLayout />}>
          <Route index element={<BooksPage />} />
          <Route path="books/:id" element={<BookDetailPage />} />
          <Route path="authors" element={<AuthorsPage />} />
          <Route path="authors/:id" element={<AuthorDetailPage />} />
          <Route path="series" element={<SeriesPage />} />
          <Route path="series/:id" element={<SeriesDetailPage />} />
          <Route path="profile" element={<ProfilePage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
