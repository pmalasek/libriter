import { RepairCard } from '@/components/admin/RepairCard'
import { ScannerCard } from '@/components/admin/ScannerCard'

export function AdminLibraryPage() {
  return (
    <div className="space-y-6">
      <ScannerCard />
      <RepairCard />
    </div>
  )
}
