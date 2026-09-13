import { Route, Routes } from 'react-router'
import { RedirectIfAuthenticated, RequireAuth } from '@/auth/RequireAuth'
import { AppLayout } from '@/components/layout/AppLayout'
import { AuthorDetailPage } from '@/pages/AuthorDetailPage'
import { AuthorsPage } from '@/pages/AuthorsPage'
import { BookDetailPage } from '@/pages/BookDetailPage'
import { BooksPage } from '@/pages/BooksPage'
import { LoginPage } from '@/pages/LoginPage'
import { NotFoundPage } from '@/pages/NotFoundPage'
import { ProfilePage } from '@/pages/ProfilePage'
import { RegisterPage } from '@/pages/RegisterPage'
import { SeriesDetailPage } from '@/pages/SeriesDetailPage'
import { SeriesPage } from '@/pages/SeriesPage'

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
