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
import { BILLING_CACHE_VAR_MAP } from './billing-expr'

export const CACHE_MODE_TIMED = 'timed'
export const CACHE_MODE_GENERIC = 'generic'
export type CacheMode = typeof CACHE_MODE_TIMED | typeof CACHE_MODE_GENERIC

export type TierConditionInput = {
  var: 'p' | 'c' | 'len'
  op: '<' | '<=' | '>' | '>='
  value: number | string
}

export type VisualTier = {
  label: string
  conditions: TierConditionInput[]
  input_unit_cost: number
  output_unit_cost: number
  cache_mode: CacheMode
  cache_read_unit_cost?: number
  cache_create_unit_cost?: number
  cache_create_1h_unit_cost?: number
  image_unit_cost?: number
  image_output_unit_cost?: number
  audio_input_unit_cost?: number
  audio_output_unit_cost?: number
  [field: string]: unknown
}

export type VisualConfig = {
  tiers: VisualTier[]
}

export function getTierCacheMode(
  tier: Partial<VisualTier> | null | undefined
): CacheMode {
  if (tier?.cache_mode === CACHE_MODE_TIMED) return CACHE_MODE_TIMED
  if (tier?.cache_mode === CACHE_MODE_GENERIC) return CACHE_MODE_GENERIC
  return Number(tier?.cache_create_1h_unit_cost) > 0
    ? CACHE_MODE_TIMED
    : CACHE_MODE_GENERIC
}

export function normalizeVisualTier(
  tier: Partial<VisualTier> = {}
): VisualTier {
  return {
    label: tier.label ?? '',
    input_unit_cost: Number(tier.input_unit_cost) || 0,
    output_unit_cost: Number(tier.output_unit_cost) || 0,
    cache_mode: getTierCacheMode(tier),
    conditions: Array.isArray(tier.conditions) ? tier.conditions : [],
    ...tier,
    cache_read_unit_cost: Number(tier.cache_read_unit_cost) || 0,
    cache_create_unit_cost: Number(tier.cache_create_unit_cost) || 0,
    cache_create_1h_unit_cost: Number(tier.cache_create_1h_unit_cost) || 0,
    image_unit_cost: Number(tier.image_unit_cost) || 0,
    image_output_unit_cost: Number(tier.image_output_unit_cost) || 0,
    audio_input_unit_cost: Number(tier.audio_input_unit_cost) || 0,
    audio_output_unit_cost: Number(tier.audio_output_unit_cost) || 0,
  }
}

export function createDefaultVisualConfig(): VisualConfig {
  return {
    tiers: [
      normalizeVisualTier({
        conditions: [],
        input_unit_cost: 0,
        output_unit_cost: 0,
        label: 'base',
        cache_mode: CACHE_MODE_GENERIC,
      }),
    ],
  }
}

export function normalizeVisualConfig(
  config: VisualConfig | null | undefined
): VisualConfig {
  if (!config || !Array.isArray(config.tiers) || config.tiers.length === 0) {
    return createDefaultVisualConfig()
  }
  return {
    ...config,
    tiers: config.tiers.map((tier) => normalizeVisualTier(tier)),
  }
}

function buildConditionStr(conditions: TierConditionInput[]): string {
  if (!conditions || conditions.length === 0) return ''
  return conditions
    .filter((c) => c.var && c.op && c.value != null && c.value !== '')
    .map((c) => `${c.var} ${c.op} ${c.value}`)
    .join(' && ')
}

function buildTierBodyExpr(tier: VisualTier): string {
  const parts: string[] = []
  const ic = Number(tier.input_unit_cost) || 0
  const oc = Number(tier.output_unit_cost) || 0
  parts.push(`p * ${ic}`)
  parts.push(`c * ${oc}`)
  for (const cv of BILLING_CACHE_VAR_MAP) {
    const v = Number((tier as Record<string, unknown>)[cv.field]) || 0
    if (v !== 0) parts.push(`${cv.exprVar} * ${v}`)
  }
  return parts.join(' + ')
}

export function generateExprFromVisualConfig(
  config: VisualConfig | null | undefined
): string {
  if (!config || !config.tiers || config.tiers.length === 0) {
    return 'p * 0 + c * 0'
  }
  const tiers = config.tiers

  if (tiers.length === 1) {
    const tier = tiers[0]
    const label = tier.label || 'default'
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)
    if (cond) {
      return `${cond} ? ${body} : p * 0 + c * 0`
    }
    return body
  }

  const parts: string[] = []
  for (let i = 0; i < tiers.length; i++) {
    const tier = tiers[i]
    const label = tier.label || `tier_${i + 1}`
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)

    if (i < tiers.length - 1 && cond) {
      parts.push(`${cond} ? ${body}`)
    } else {
      parts.push(body)
    }
  }
  return parts.join(' : ')
}

export function tryParseVisualConfig(
  exprStr: string | null | undefined
): VisualConfig | null {
  if (!exprStr) return null
  try {
    let body = exprStr
    const versionMatch = body.match(/^v\d+:([\s\S]*)$/)
    if (versionMatch) body = versionMatch[1]
    const cacheVarNames = BILLING_CACHE_VAR_MAP.map((cv) => cv.exprVar)
    const optCacheStr = cacheVarNames
      .map((v) => `(?:\\s*\\+\\s*${v}\\s*\\*\\s*([\\d.eE+-]+))?`)
      .join('')

    const bodyPat = `p\\s*\\*\\s*([\\d.eE+-]+)\\s*\\+\\s*c\\s*\\*\\s*([\\d.eE+-]+)${optCacheStr}`

    const singleRe = new RegExp(`^tier\\("([^"]*)",\\s*${bodyPat}\\)$`)
    const simple = body.match(singleRe)
    if (simple) {
      const tier: Record<string, unknown> = {
        conditions: [],
        input_unit_cost: Number(simple[2]),
        output_unit_cost: Number(simple[3]),
        label: simple[1],
      }
      BILLING_CACHE_VAR_MAP.forEach((cv, i) => {
        const val = simple[4 + i]
        if (val != null) tier[cv.field] = Number(val)
      })
      return normalizeVisualConfig({
        tiers: [normalizeVisualTier(tier as Partial<VisualTier>)],
      })
    }

    const condGroup =
      `((?:(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)` +
      `(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)*)`
    const tierRe = new RegExp(
      `(?:${condGroup}\\s*\\?\\s*)?tier\\("([^"]*)",\\s*${bodyPat}\\)`,
      'g'
    )
    const tiers: VisualTier[] = []
    let match: RegExpExecArray | null
    while ((match = tierRe.exec(body)) !== null) {
      const condStr = match[1] || ''
      const conditions: TierConditionInput[] = []
      if (condStr) {
        for (const cp of condStr.split(/\s*&&\s*/)) {
          const cm = cp.trim().match(/^(p|c|len)\s*(<|<=|>|>=)\s*([\d.eE+]+)$/)
          if (cm) {
            conditions.push({
              var: cm[1] as TierConditionInput['var'],
              op: cm[2] as TierConditionInput['op'],
              value: Number(cm[3]),
            })
          }
        }
      }
      const tier: Record<string, unknown> = {
        conditions,
        input_unit_cost: Number(match[3]),
        output_unit_cost: Number(match[4]),
        label: match[2],
      }
      const m = match
      BILLING_CACHE_VAR_MAP.forEach((cv, i) => {
        const val = m[5 + i]
        if (val != null) tier[cv.field] = Number(val)
      })
      tiers.push(normalizeVisualTier(tier as Partial<VisualTier>))
    }
    if (tiers.length === 0) return null

    const cfg = normalizeVisualConfig({ tiers })
    const regenerated = generateExprFromVisualConfig(cfg)
    if (regenerated.replace(/\s+/g, '') !== body.replace(/\s+/g, '')) {
      return null
    }
    return cfg
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// Local cost evaluator (for the estimator preview)
// ---------------------------------------------------------------------------

const ESTIMATOR_VARS = [
  { var: 'cr', stateKey: 'cacheReadTokens' },
  { var: 'cc', stateKey: 'cacheCreateTokens' },
  { var: 'cc1h', stateKey: 'cacheCreate1hTokens' },
  { var: 'img', stateKey: 'imageTokens' },
  { var: 'img_o', stateKey: 'imageOutputTokens' },
  { var: 'ai', stateKey: 'audioInputTokens' },
  { var: 'ao', stateKey: 'audioOutputTokens' },
] as const

export type ExtraTokenValues = Record<
  (typeof ESTIMATOR_VARS)[number]['stateKey'],
  number
>

export type EvalResult = {
  cost: number
  matchedTier: string
  error: string | null
}

export function evalExprLocally(
  exprStr: string,
  promptTokens: number,
  completionTokens: number,
  extraTokenValues: ExtraTokenValues
): EvalResult {
  try {
    if (!exprStr || !exprStr.trim()) {
      return { cost: 0, matchedTier: '', error: null }
    }
    let matchedTier = ''
    const tierFn = (name: string, value: number) => {
      matchedTier = name
      return value
    }
    const cacheReadTokens = extraTokenValues.cacheReadTokens || 0
    const cacheCreateTokens = extraTokenValues.cacheCreateTokens || 0
    const cacheCreate1hTokens = extraTokenValues.cacheCreate1hTokens || 0
    const len =
      promptTokens + cacheReadTokens + cacheCreateTokens + cacheCreate1hTokens
    const env: Record<string, unknown> = {
      p: promptTokens,
      c: completionTokens,
      len,
      tier: tierFn,
      max: Math.max,
      min: Math.min,
      abs: Math.abs,
      ceil: Math.ceil,
      floor: Math.floor,
    }
    for (const field of ESTIMATOR_VARS) {
      env[field.var] = extraTokenValues[field.stateKey] || 0
    }
    const fn = new Function(
      ...Object.keys(env),
      `"use strict"; return (${exprStr});`
    )
    const cost = Number(fn(...Object.values(env))) || 0
    return { cost, matchedTier, error: null }
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e)
    return { cost: 0, matchedTier: '', error: message }
  }
}

export function exprUsesExtraVars(exprStr: string): boolean {
  if (!exprStr) return false
  const varNames = ESTIMATOR_VARS.map((f) => f.var).join('|')
  return new RegExp(`\\b(${varNames})\\b`).test(exprStr)
}

export const ESTIMATOR_EXTRA_FIELDS = ESTIMATOR_VARS

// ---------------------------------------------------------------------------
// Per-call visual config (v2: prefix)
// ---------------------------------------------------------------------------

/** 按次计费的单条规则中的 param 条件：param(path) == value */
export type PerCallParamCondition = {
  path: string   // gjson 路径，如 "metadata.modelEdition"
  value: string  // 等值匹配
}

/** 按次计费的单条规则（必须有至少一个条件） */
export type PerCallRule = {
  label: string
  conditions: PerCallParamCondition[]
  pricePerCall: number  // $/次
}

/**
 * Per-call 视觉配置：
 * - rules: 有条件的规则列表，多条命中时取价格最高的
 * - fallbackPrice: 兜底价，始终参与竞价
 */
export type PerCallVisualConfig = {
  base: 'per_call'
  rules: PerCallRule[]
  fallbackPrice: number
}

export function createDefaultPerCallConfig(): PerCallVisualConfig {
  return {
    base: 'per_call',
    rules: [],
    fallbackPrice: 0,
  }
}

export function normalizePerCallRule(rule: Partial<PerCallRule>): PerCallRule {
  return {
    label: rule.label ?? '',
    conditions: Array.isArray(rule.conditions) ? rule.conditions : [],
    pricePerCall: Number(rule.pricePerCall) || 0,
  }
}

/**
 * 生成 v2: 按次计费表达式，多条命中取最高价（nested max）。
 *
 * 无规则：
 *   v2: tier("base", 0.06)
 *
 * 单规则：
 *   v2: tier("result", max(param("metadata.modelEdition") == 3 ? 0.15 : 0, 0.06))
 *
 * 多规则（嵌套 max）：
 *   v2: tier("result", max(
 *     param("metadata.modelEdition") == 3 ? 0.15 : 0,
 *     max(param("metadata.modelEdition") == 2 ? 0.09 : 0, 0.06)
 *   ))
 */
export function generateExprFromPerCallConfig(
  config: PerCallVisualConfig | null | undefined
): string {
  const fallback = config?.fallbackPrice ?? 0
  const rules = (config?.rules ?? []).filter((r) => r.conditions.length > 0)

  // 始终输出浮点格式，避免解析时 int/float 不匹配
  const fmtNum = (n: number) => Number.isInteger(n) ? `${n}.0` : String(n)

  if (rules.length === 0) {
    return `v2: tier("base", ${fmtNum(fallback)})`
  }

  function buildCondStr(rule: PerCallRule): string {
    const parts = rule.conditions
      .filter((c) => c.path.trim() !== '')
      .map((c) => {
        const numVal = Number(c.value)
        const valStr =
          !isNaN(numVal) && c.value.trim() !== ''
            ? (Number.isInteger(numVal) ? `${numVal}.0` : String(numVal))
            : `"${c.value}"`
        return `param("${c.path}") == ${valStr}`
      })
    return parts.join(' && ')
  }

  function buildMaxChain(index: number): string {
    const rule = rules[index]
    const cond = buildCondStr(rule)
    const price = fmtNum(rule.pricePerCall)
    const term = cond ? `${cond} ? ${price} : 0.0` : price

    if (index === rules.length - 1) {
      return `max(${term}, ${fmtNum(fallback)})`
    }
    return `max(${term}, ${buildMaxChain(index + 1)})`
  }

  return `v2: tier("result", ${buildMaxChain(0)})`
}

/** 反向解析 v2: 表达式回 PerCallVisualConfig，解析失败返回 null */
export function tryParsePerCallConfig(
  exprStr: string | null | undefined
): PerCallVisualConfig | null {
  if (!exprStr) return null
  const trimmed = exprStr.trim()
  if (!trimmed.startsWith('v2:')) return null

  try {
    const body = trimmed.slice(3).trim()

    // 无规则形式: tier("base", price)
    const baseRe = /^tier\("([^"]*)",\s*(-?[\d.eE+]+)\)$/
    const bm = body.match(baseRe)
    if (bm) {
      return { base: 'per_call', rules: [], fallbackPrice: Number(bm[2]) }
    }

    // 有规则形式: tier("result", max(...))
    const outerRe = /^tier\("result",\s*([\s\S]+)\)$/
    const om = body.match(outerRe)
    if (!om) return null

    const rules: PerCallRule[] = []
    let fallbackPrice = 0

    // 递归解析嵌套 max
    function parseMax(s: string): void {
      s = s.trim()
      // max(term, rest)
      if (!s.startsWith('max(')) return
      const inner = s.slice(4, -1).trim()

      // 找到第一个逗号（跳过嵌套括号）
      let depth = 0
      let splitIdx = -1
      for (let i = 0; i < inner.length; i++) {
        if (inner[i] === '(') depth++
        else if (inner[i] === ')') depth--
        else if (inner[i] === ',' && depth === 0) {
          splitIdx = i
          break
        }
      }
      if (splitIdx === -1) return

      const termStr = inner.slice(0, splitIdx).trim()
      const restStr = inner.slice(splitIdx + 1).trim()

      // 解析 term: cond ? price : 0.0
      const termRe =
        /^((?:param\("[^"]+"\)\s*==\s*(?:"[^"]*"|-?[\d.]+)(?:\s*&&\s*param\("[^"]+"\)\s*==\s*(?:"[^"]*"|-?[\d.]+))*)\s*\?\s*(-?[\d.eE+]+)\s*:\s*0\.0)$/
      const tm = termStr.match(termRe)
      if (tm) {
        const condStr = tm[1].split('?')[0].trim()
        const price = Number(tm[2])
        const conditions: PerCallParamCondition[] = []
        for (const part of condStr.split(/\s*&&\s*/)) {
          const cm = part
            .trim()
            .match(
              /^param\("([^"]+)"\)\s*==\s*(?:"([^"]*)"|([\d.eE+-]+))$/
            )
          if (cm) {
            conditions.push({ path: cm[1], value: cm[2] ?? cm[3] })
          }
        }
        rules.push({ label: `rule_${rules.length + 1}`, conditions, pricePerCall: price })
      }

      // rest 是下一层 max 或 fallback 数字
      if (restStr.startsWith('max(')) {
        parseMax(restStr)
      } else {
        fallbackPrice = Number(restStr) || 0
      }
    }

    parseMax(om[1].trim())

    const cfg: PerCallVisualConfig = { base: 'per_call', rules, fallbackPrice }
    const regenerated = generateExprFromPerCallConfig(cfg)
    if (regenerated.replace(/\s+/g, '') !== trimmed.replace(/\s+/g, '')) {
      return null
    }
    return cfg
  } catch {
    return null
  }
}

/** per-call 模式下的本地单次费用估算 */
export function evalPerCallExprLocally(
  exprStr: string,
  paramValues: Record<string, unknown>
): EvalResult {
  try {
    if (!exprStr || !exprStr.trim()) {
      return { cost: 0, matchedTier: '', error: null }
    }
    let matchedTier = ''
    const tierFn = (name: string, value: number) => {
      matchedTier = name
      return value
    }
    const paramFn = (path: string) => {
      const parts = path.split('.')
      let cur: unknown = paramValues
      for (const p of parts) {
        if (cur == null || typeof cur !== 'object') return null
        cur = (cur as Record<string, unknown>)[p]
      }
      return cur ?? null
    }
    const env: Record<string, unknown> = {
      tier: tierFn,
      param: paramFn,
      max: Math.max,
      min: Math.min,
      abs: Math.abs,
      ceil: Math.ceil,
      floor: Math.floor,
    }
    let body = exprStr.trim()
    if (body.startsWith('v2:')) body = body.slice(3).trim()

    const fn = new Function(
      ...Object.keys(env),
      `"use strict"; return (${body});`
    )
    const cost = Number(fn(...Object.values(env))) || 0
    return { cost, matchedTier, error: null }
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e)
    return { cost: 0, matchedTier: '', error: message }
  }
}
