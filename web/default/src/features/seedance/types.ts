export interface SeedanceAssetGroup {
  id: number
  user_id: number
  channel_id: number
  upstream_group_id: string
  name: string
  description: string
  group_type: string // AIGC or LivenessFace
  raw_data: string
  created_at: number
  updated_at: number
  deleted_at: number
}

export interface SeedanceAsset {
  id: number
  user_id: number
  channel_id: number
  upstream_asset_id: string
  upstream_group_id: string
  name: string
  asset_type: string // Image / Video / Audio
  source_url: string
  status: string // Processing / Active / Failed
  raw_data: string
  created_at: number
  updated_at: number
  deleted_at: number
}

export interface SeedanceFaceVerification {
  id: number
  user_id: number
  channel_id: number
  verification_id: string
  status: string // waiting_user/callback_received/resolving/verified/failed/expired
  h5_url: string
  group_id: string
  expires_at: number
  raw_data: string
  created_at: number
  updated_at: number
  deleted_at: number
}

export interface PagedResponse<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}
