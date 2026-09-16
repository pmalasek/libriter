import { useEffect, useLayoutEffect, useState } from 'react'
import { ColorSchemeContext, parseScheme, readScheme, STORAGE_KEY } from './colorScheme'

export function ColorSchemeProvider({ children }: { children: React.ReactNode }) {
  const [colorScheme, setScheme] = useState(readScheme)

  useLayoutEffect(() => {
    document.documentElement.dataset.colorScheme = colorScheme
  }, [colorScheme])

  useEffect(() => {
    const syncScheme = (event: StorageEvent) => {
      if (event.storageArea === localStorage && (event.key === STORAGE_KEY || event.key === null)) {
        setScheme(parseScheme(event.newValue))
      }
    }
    window.addEventListener('storage', syncScheme)
    return () => window.removeEventListener('storage', syncScheme)
  }, [])

  function setColorScheme(value: string) {
    const next = parseScheme(value)
    setScheme(next)
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // Vzhled lze změnit i tehdy, když prohlížeč nepovoluje ukládání.
    }
  }

  return (
    <ColorSchemeContext.Provider value={{ colorScheme, setColorScheme }}>
      {children}
    </ColorSchemeContext.Provider>
  )
}

