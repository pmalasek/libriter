import { DuplicatesCard } from '@/components/admin/DuplicatesCard'
import { ImportCard } from '@/components/admin/ImportCard'
import { LibrarySettingsCard } from '@/components/admin/LibrarySettingsCard'
import { MergeCard } from '@/components/admin/MergeCard'
import { RepairCard } from '@/components/admin/RepairCard'
import { ScannerCard } from '@/components/admin/ScannerCard'

export function AdminLibraryPage() {
  return (
    <div className="space-y-6">
      <LibrarySettingsCard />
      <ImportCard />
      <ScannerCard />
      <RepairCard />
      <DuplicatesCard />
      <MergeCard />
    </div>
  )
}
