import { api } from '@/lib/api'
import type {
  SeedanceAssetGroup,
  SeedanceAsset,
  SeedanceFaceVerification,
  PagedResponse,
} from './types'

interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
}

// ---- Admin: Asset Groups ----

export async function adminListAssetGroups(params: {
  p?: number
  page_size?: number
  user_id?: number
}): Promise<PagedResponse<SeedanceAssetGroup>> {
  const res = await api.get<ApiResponse<PagedResponse<SeedanceAssetGroup>>>(
    '/api/admin/seedance/asset-groups',
    { params }
  )
  return res.data.data
}

// ---- Admin: Assets ----

export async function adminListAssets(params: {
  p?: number
  page_size?: number
  user_id?: number
  group_id?: string
}): Promise<PagedResponse<SeedanceAsset>> {
  const res = await api.get<ApiResponse<PagedResponse<SeedanceAsset>>>(
    '/api/admin/seedance/assets',
    { params }
  )
  return res.data.data
}

// ---- User: Asset Groups ----

export async function userListAssetGroups(params: {
  p?: number
  page_size?: number
}): Promise<PagedResponse<SeedanceAssetGroup>> {
  const res = await api.get<ApiResponse<PagedResponse<SeedanceAssetGroup>>>(
    '/api/seedance/asset-groups',
    { params }
  )
  return res.data.data
}

// ---- User: Assets ----

export async function userListAssets(params: {
  p?: number
  page_size?: number
  group_id?: string
}): Promise<PagedResponse<SeedanceAsset>> {
  const res = await api.get<ApiResponse<PagedResponse<SeedanceAsset>>>(
    '/api/seedance/assets',
    { params }
  )
  return res.data.data
}

// ---- Admin: Face Verifications ----

export async function adminListFaceVerifications(params: {
  p?: number
  page_size?: number
  user_id?: number
}): Promise<PagedResponse<SeedanceFaceVerification>> {
  const res = await api.get<
    ApiResponse<PagedResponse<SeedanceFaceVerification>>
  >('/api/admin/seedance/face-verifications', { params })
  return res.data.data
}

// ---- User: Face Verifications ----

export async function userListFaceVerifications(params: {
  p?: number
  page_size?: number
}): Promise<PagedResponse<SeedanceFaceVerification>> {
  const res = await api.get<
    ApiResponse<PagedResponse<SeedanceFaceVerification>>
  >('/api/seedance/face-verifications', { params })
  return res.data.data
}

export const seedanceQueryKeys = {
  all: ['seedance'] as const,
  assetGroups: (params?: object) =>
    [...seedanceQueryKeys.all, 'asset-groups', params] as const,
  assets: (params?: object) =>
    [...seedanceQueryKeys.all, 'assets', params] as const,
  faceVerifications: (params?: object) =>
    [...seedanceQueryKeys.all, 'face-verifications', params] as const,
}
