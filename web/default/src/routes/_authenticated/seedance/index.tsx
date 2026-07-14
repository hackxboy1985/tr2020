import { createFileRoute } from '@tanstack/react-router'
import { SeedanceAssets } from '@/features/seedance'

export const Route = createFileRoute('/_authenticated/seedance/')({
  component: SeedanceAssets,
})
