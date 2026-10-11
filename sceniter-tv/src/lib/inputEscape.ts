import { useRef } from 'react'
import { useTVEventHandler, type View } from 'react-native'

/**
 * Šipka dolů z textového pole. Android TV `EditText` šipku pohltí a fokus
 * z něj sám neodejde – pole by bylo slepá ulička. Dokud má pole fokus,
 * šipka dolů (uvolnění klávesy) přesune fokus na `target`.
 *
 * Vrací handlery onFocus/onBlur pro TextInput.
 */
export function useDownFromInput(target: () => View | null | undefined) {
  const focused = useRef(false)
  const targetRef = useRef(target)
  targetRef.current = target
  useTVEventHandler((e) => {
    if (e.eventType !== 'down' || e.eventKeyAction === 0 || !focused.current) return
    targetRef.current()?.requestTVFocus()
  })
  return {
    onFocus: () => (focused.current = true),
    onBlur: () => (focused.current = false),
  }
}
