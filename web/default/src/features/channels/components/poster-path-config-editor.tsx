/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useRef, useState } from 'react'
import { Code, Plus, Table, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

const POSTER_MODELS = [
  'poster-extension',
  'poster-translate',
  'poster-enlarge',
  'poster-matting',
  'poster-enhance',
  'poster-partial-redraw',
  'poster-scene-replace',
  'poster-product-replace',
  'poster-color-change',
  'poster-assisted',
  'poster-generate',
  'poster-free-creation',
]

type EndpointRow = {
  id: string
  model: string
  path: string
}

type PosterPathConfig = {
  poster_api_version?: string
  poster_endpoints?: Record<string, string>
}

type PosterPathConfigEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

export function PosterPathConfigEditor(props: PosterPathConfigEditorProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [apiVersion, setApiVersion] = useState('')
  const [rows, setRows] = useState<EndpointRow[]>([])
  const [jsonValue, setJsonValue] = useState(props.value)
  const [jsonError, setJsonError] = useState<string | null>(null)
  const nextRowIdRef = useRef(0)

  const createRowId = () => {
    nextRowIdRef.current += 1
    return `endpoint-${nextRowIdRef.current}`
  }

  const buildJson = (version: string, endpointRows: EndpointRow[]): string => {
    const config: PosterPathConfig = {}
    if (version.trim()) {
      config.poster_api_version = version.trim()
    }
    const endpoints: Record<string, string> = {}
    for (const row of endpointRows) {
      if (row.model.trim() && row.path.trim()) {
        endpoints[row.model.trim()] = row.path.trim()
      }
    }
    if (Object.keys(endpoints).length > 0) {
      config.poster_endpoints = endpoints
    }
    if (Object.keys(config).length === 0) {
      return ''
    }
    return JSON.stringify(config, null, 2)
  }

  const parseJson = (json: string): boolean => {
    try {
      if (!json.trim()) {
        setApiVersion('')
        setRows([])
        setJsonError(null)
        return true
      }
      const parsed: PosterPathConfig = JSON.parse(json)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        setJsonError(t('Must be a valid JSON object'))
        return false
      }
      setApiVersion(parsed.poster_api_version ?? '')
      const endpoints = parsed.poster_endpoints ?? {}
      setRows(
        Object.entries(endpoints).map(([model, path]) => ({
          id: createRowId(),
          model,
          path,
        }))
      )
      setJsonError(null)
      return true
    } catch (_e) {
      setJsonError(t('Invalid JSON format'))
      return false
    }
  }

  useEffect(() => {
    setJsonValue(props.value)
    parseJson(props.value)
  }, [props.value])

  const syncVisual = (version: string, endpointRows: EndpointRow[]) => {
    const json = buildJson(version, endpointRows)
    setJsonValue(json)
    setJsonError(null)
    props.onChange(json)
  }

  const handleVersionChange = (v: string) => {
    setApiVersion(v)
    syncVisual(v, rows)
  }

  const handleAddRow = () => {
    const newRows = [...rows, { id: createRowId(), model: '', path: '' }]
    setRows(newRows)
    syncVisual(apiVersion, newRows)
  }

  const handleDeleteRow = (id: string) => {
    const newRows = rows.filter((r) => r.id !== id)
    setRows(newRows)
    syncVisual(apiVersion, newRows)
  }

  const handleRowChange = (
    id: string,
    field: 'model' | 'path',
    value: string
  ) => {
    const newRows = rows.map((r) => (r.id === id ? { ...r, [field]: value } : r))
    setRows(newRows)
    syncVisual(apiVersion, newRows)
  }

  const handleJsonChange = (newJson: string) => {
    setJsonValue(newJson)
    props.onChange(newJson)
    parseJson(newJson)
  }

  const handleModeChange = (nextMode: string) => {
    if (nextMode !== 'visual' && nextMode !== 'json') return
    if (nextMode === 'json') {
      const json = buildJson(apiVersion, rows)
      setJsonValue(json)
      props.onChange(json)
    } else {
      parseJson(jsonValue)
    }
    setMode(nextMode)
  }

  return (
    <div className='space-y-2'>
      <Tabs value={mode} onValueChange={handleModeChange} className='space-y-2'>
        <TabsList>
          <TabsTrigger value='visual'>
            <Table className='h-4 w-4' aria-hidden='true' />
            {t('Visual')}
          </TabsTrigger>
          <TabsTrigger value='json'>
            <Code className='h-4 w-4' aria-hidden='true' />
            {t('JSON')}
          </TabsTrigger>
        </TabsList>

        {jsonError && (
          <Alert variant='destructive'>
            <AlertDescription>{jsonError}</AlertDescription>
          </Alert>
        )}

        <TabsContent value='visual' className='space-y-4'>
          {/* API Version */}
          <div className='space-y-1'>
            <div className='text-sm font-medium'>{t('API Version')}</div>
            <Input
              value={apiVersion}
              onChange={(e) => handleVersionChange(e.target.value)}
              placeholder='v2'
              disabled={props.disabled}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'Replaces /v1/ in all default paths. e.g. "v2" → /openapi/v2/...'
              )}
            </p>
          </div>

          {/* Endpoint Overrides */}
          <div className='space-y-2'>
            <div className='text-sm font-medium'>{t('Endpoint Overrides')}</div>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Override specific model paths. Higher priority than API Version.'
              )}
            </p>
            {rows.length > 0 ? (
              <div className='space-y-2'>
                <div className='grid grid-cols-[1fr_1fr_auto] gap-2 text-sm font-medium'>
                  <div>{t('Model')}</div>
                  <div>{t('Path')}</div>
                  <div className='w-10' />
                </div>
                {rows.map((row) => (
                  <div
                    key={row.id}
                    className='grid grid-cols-[1fr_1fr_auto] gap-2'
                  >
                    <Input
                      value={row.model}
                      onChange={(e) =>
                        handleRowChange(row.id, 'model', e.target.value)
                      }
                      placeholder='poster-matting'
                      disabled={props.disabled}
                      list='poster-model-list'
                    />
                    <Input
                      value={row.path}
                      onChange={(e) =>
                        handleRowChange(row.id, 'path', e.target.value)
                      }
                      placeholder='/openapi/v2/ai/matting_pro'
                      disabled={props.disabled}
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      onClick={() => handleDeleteRow(row.id)}
                      disabled={props.disabled}
                      className='h-10 w-10'
                      aria-label={t('Delete')}
                    >
                      <Trash2 className='h-4 w-4' aria-hidden='true' />
                    </Button>
                  </div>
                ))}
              </div>
            ) : (
              <div className='text-muted-foreground flex h-20 items-center justify-center rounded-md border border-dashed text-sm'>
                {t('No endpoint overrides. Click "Add Override" to add one.')}
              </div>
            )}
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={handleAddRow}
              disabled={props.disabled}
              className='w-full'
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add Override')}
            </Button>
          </div>
        </TabsContent>

        <TabsContent value='json'>
          <Textarea
            value={jsonValue}
            onChange={(e) => handleJsonChange(e.target.value)}
            placeholder={'{\n  "poster_api_version": "v2",\n  "poster_endpoints": {\n    "poster-matting": "/openapi/v2/ai/matting_pro"\n  }\n}'}
            disabled={props.disabled}
            rows={8}
            className={cn('font-mono text-sm', jsonError && 'border-destructive')}
            aria-invalid={Boolean(jsonError)}
          />
        </TabsContent>
      </Tabs>

      <datalist id='poster-model-list'>
        {POSTER_MODELS.map((m) => (
          <option key={m} value={m} />
        ))}
      </datalist>
    </div>
  )
}
