import { ChevronDownIcon } from 'lucide-react'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { NowPlayingPanel } from './NowPlayingPanel'

/**
 * Rozbalený přehrávač pro telefon a tablet. Na úzké obrazovce se do kapsle
 * vejde jen název a pár tlačítek, zbytek ovládání i celá informace o knize
 * je tady – panel se vytáhne klepnutím na kapsli.
 *
 * Obsah je stejný jako ve sloupci na širokém displeji (NowPlayingPanel).
 */
export function PlayerSheet({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="bottom"
        showCloseButton={false}
        // Celá výška: panel má vlastní rolovací blok s kapitolami, u nižšího
        // panelu by se do něj nevešly ani ovládací prvky. Výšku je nutné
        // přepsat se stejnou variantou – základní třída má
        // `data-[side=bottom]:h-auto` a prosté `h-*` na ni nestačí.
        className="gap-0 rounded-none px-0 pt-2 pb-[max(0.5rem,env(safe-area-inset-bottom))] data-[side=bottom]:h-[100svh]"
      >
        <SheetHeader className="sr-only p-0">
          <SheetTitle>Přehrávač</SheetTitle>
        </SheetHeader>

        {/* Šipka dolů panel zase sbalí zpátky do kapsle. */}
        <button
          type="button"
          onClick={() => onOpenChange(false)}
          className="flex w-full shrink-0 items-center justify-center py-1 text-muted-foreground transition-colors hover:text-foreground"
          aria-label="Sbalit přehrávač"
        >
          <ChevronDownIcon className="size-5" />
        </button>

        <NowPlayingPanel onNavigate={() => onOpenChange(false)} />
      </SheetContent>
    </Sheet>
  )
}
