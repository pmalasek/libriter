import { ImageUpIcon } from 'lucide-react'
import { useRef } from 'react'
import { Button } from '@/components/ui/button'

// Formáty, které backend přijme (typ si ověří podle obsahu).
const ACCEPT = 'image/jpeg,image/png,image/webp,image/gif,image/bmp'

/** Tlačítko, které otevře výběr obrázku a předá vybraný soubor. */
export function ImageUploadButton({
  label,
  disabled,
  onFile,
}: {
  label: string
  disabled?: boolean
  onFile: (file: File) => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        disabled={disabled}
        onClick={() => inputRef.current?.click()}
      >
        <ImageUpIcon />
        {label}
      </Button>
      <input
        ref={inputRef}
        type="file"
        hidden
        accept={ACCEPT}
        onChange={(event) => {
          const file = event.target.files?.[0]
          // Reset, aby šlo stejný soubor vybrat znovu.
          event.target.value = ''
          if (file) onFile(file)
        }}
      />
    </>
  )
}
