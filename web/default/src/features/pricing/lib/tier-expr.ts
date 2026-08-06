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

/** 乘数操作符 */
export type PerCallMultiplierOp = 'none' | '*' | '+' | '-' | '/'

/** 乘数来源类型 */
export type PerCallMultiplierFieldType = 'param' | 'number'

/** 乘数配置 */
export type PerCallMultiplier = {
  op: PerCallMultiplierOp
  fieldType: PerCallMultiplierFieldType
  field: string       // 请求体字段路径或自然数值
  fallback: string    // 字段获取失败时的默认值（fieldType=param 时有效）
}

export function createDefaultMultiplier(): PerCallMultiplier {
  return { op: 'none', fieldType: 'param', field: '', fallback: '1' }
}

/** 按次计费的单条规则（必须有至少一个条件） */
export type PerCallRule = {
  label: string
  conditions: PerCallParamCondition[]
  pricePerCall: number  // $/次
  multiplier: PerCallMultiplier
}

/**
 * Per-call 视觉配置：
 * - rules: 有条件的规则列表，多条命中时取价格最高的
 * - fallbackPrice: 兜底价，始终参与竞价
 * - fallbackMultiplier: 兜底价的乘数配置
 */
export type PerCallVisualConfig = {
  base: 'per_call'
  rules: PerCallRule[]
  fallbackPrice: number
  fallbackMultiplier: PerCallMultiplier
}

export function createDefaultPerCallConfig(): PerCallVisualConfig {
  return {
    base: 'per_call',
    rules: [],
    fallbackPrice: 0,
    fallbackMultiplier: createDefaultMultiplier(),
  }
}

export function normalizePerCallRule(rule: Partial<PerCallRule>): PerCallRule {
  return {
    label: rule.label ?? '',
    conditions: Array.isArray(rule.conditions) ? rule.conditions : [],
    pricePerCall: Number(rule.pricePerCall) || 0,
    multiplier: rule.multiplier ?? createDefaultMultiplier(),
  }
}

/** 构建乘数表达式片段，如 " * max(isnull(param("f"), 0.0), 1)" 或 " * 3" */
export function buildMultiplierExpr(mul: PerCallMultiplier): string {
  if (!mul || mul.op === 'none') return ''
  const op = mul.op
  if (mul.fieldType === 'number') {
    const n = Number(mul.field)
    if (isNaN(n) || mul.field.trim() === '') return ''
    return ` ${op} ${n}`
  }
  // param 类型: max(isnull(param("field"), 0.0), fallback)
  const field = mul.field.trim()
  if (!field) return ''
  const fb = Number(mul.fallback)
  const safeFb = isNaN(fb) ? 1 : (op === '/' && fb === 0 ? 1 : fb)
  return ` ${op} max(isnull(param("${field}"), 0.0), ${safeFb})`
}

/**
 * 生成 v2: 按次计费表达式，多条命中取最高价（nested max）。
 *
 * 无规则：
 *   v2: tier("base", 0.06)
 *
 * 单规则（含乘数）：
 *   v2: tier("result", max(param("metadata.modelEdition") == 3 ? 0.15 * max(isnull(param("metadata.detailPictureNumber"), 0.0), 1) : 0.0, 0.06))
 *
 * 多规则（嵌套 max）
 */
export function generateExprFromPerCallConfig(
  config: PerCallVisualConfig | null | undefined
): string {
  const fallback = config?.fallbackPrice ?? 0
  const fallbackMul = config?.fallbackMultiplier ?? createDefaultMultiplier()
  const rules = (config?.rules ?? []).filter((r) => r.conditions.length > 0)

  // 始终输出浮点格式，避免解析时 int/float 不匹配
  const fmtNum = (n: number) => Number.isInteger(n) ? `${n}.0` : String(n)

  const fallbackExpr = `${fmtNum(fallback)}${buildMultiplierExpr(fallbackMul)}`

  if (rules.length === 0) {
    return `v2: tier("base", ${fallbackExpr})`
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

  function buildRuleExpr(index: number): string {
    const rule = rules[index]
    const cond = buildCondStr(rule)
    const price = fmtNum(rule.pricePerCall)
    const mulExpr = buildMultiplierExpr(rule.multiplier ?? createDefaultMultiplier())
    const priceWithMul = `${price}${mulExpr}`
    const ruleLabel = cond
      .replace(/param\("(?:[^"]+\.)?([^".]+)"\)\s*==\s*"([^"]*)"/g, '$1 == $2')
      .replace(/param\("(?:[^"]+\.)?([^".]+)"\)\s*==\s*([\d.]+)/g, '$1 == $2')
    const matched = `tier("${ruleLabel}", ${priceWithMul})`
    if (index === rules.length - 1) {
      return `${cond} ? ${matched} : tier("fallback", ${fallbackExpr})`
    }
    return `${cond} ? ${matched} : ${buildRuleExpr(index + 1)}`
  }

  return `v2: ${buildRuleExpr(0)}`
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

    // 解析价格+乘数片段，如 "1.0 * max(isnull(param("f"), 0.0), 1)" 或 "1.0 + 3" 或 "1.0"
    function parsePriceAndMul(s: string): { price: number; multiplier: PerCallMultiplier } | null {
      s = s.trim()
      // param 类型乘数: <price> <op> max(isnull(param("<field>"), 0.0), <fallback>)
      const paramMulRe = /^(-?[\d.eE+]+)\s*([+\-*/])\s*max\(isnull\(param\("([^"]+)"\)\s*,\s*0\.0\)\s*,\s*([\d.eE+]+)\)$/
      const pm = s.match(paramMulRe)
      if (pm) {
        return {
          price: Number(pm[1]),
          multiplier: { op: pm[2] as PerCallMultiplierOp, fieldType: 'param', field: pm[3], fallback: pm[4] },
        }
      }
      // number 类型乘数: <price> <op> <number>
      const numMulRe = /^(-?[\d.eE+]+)\s*([+\-*/])\s*([\d.eE+]+)$/
      const nm = s.match(numMulRe)
      if (nm) {
        return {
          price: Number(nm[1]),
          multiplier: { op: nm[2] as PerCallMultiplierOp, fieldType: 'number', field: nm[3], fallback: '1' },
        }
      }
      // 纯数字
      const plain = s.match(/^-?[\d.eE+]+$/)
      if (plain) {
        return { price: Number(s), multiplier: createDefaultMultiplier() }
      }
      return null
    }

    // 无规则形式: tier("base", priceExpr) — 注意不能匹配 tier("result", ...)
    const baseRe = /^tier\("base",\s*([\s\S]+)\)$/
    const bm = body.match(baseRe)
    if (bm) {
      const parsed = parsePriceAndMul(bm[1])
      if (!parsed) return null
      return {
        base: 'per_call',
        rules: [],
        fallbackPrice: parsed.price,
        fallbackMultiplier: parsed.multiplier,
      }
    }

    const rules: PerCallRule[] = []
    let fallbackPrice = 0
    let fallbackMultiplier: PerCallMultiplier = createDefaultMultiplier()

    function parseConditionText(condStr: string): PerCallParamCondition[] {
      return condStr.split(/\s*&&\s*/).flatMap((part) => {
        const cm = part.trim().match(/^param\("([^"]+)"\)\s*==\s*(?:"([^"]*)"|([\d.eE+-]+))$/)
        return cm ? [{ path: cm[1], value: cm[2] ?? cm[3] }] : []
      })
    }

    function parseRuleChain(s: string): void {
      const question = s.indexOf('?')
      if (question < 0) {
        const fallback = s.match(/^tier\("fallback",\s*([\s\S]+)\)$/)
        const parsed = parsePriceAndMul(fallback ? fallback[1] : s)
        if (parsed) {
          fallbackPrice = parsed.price
          fallbackMultiplier = parsed.multiplier
        }
        return
      }

      const condition = s.slice(0, question).trim()
      const tierStart = s.indexOf('tier("', question)
      if (tierStart < 0) return
      const labelStart = tierStart + 6
      let labelEnd = labelStart
      let escaped = false
      for (; labelEnd < s.length; labelEnd++) {
        const ch = s[labelEnd]
        if (escaped) {
          escaped = false
        } else if (ch === '\\') {
          escaped = true
        } else if (ch === '"') {
          break
        }
      }
      if (labelEnd >= s.length) return
      const bodyStart = s.indexOf(',', labelEnd + 1)
      if (bodyStart < 0) return
      let depth = 1
      let bodyEnd = bodyStart + 1
      for (; bodyEnd < s.length; bodyEnd++) {
        if (s[bodyEnd] === '(') depth++
        else if (s[bodyEnd] === ')') {
          depth--
          if (depth === 0) break
        }
      }
      if (depth !== 0) return
      const parsed = parsePriceAndMul(s.slice(bodyStart + 1, bodyEnd).trim())
      if (!parsed) return
      const label = s.slice(labelStart, labelEnd).replace(/\\"/g, '"').replace(/\\\\/g, '\\')
      rules.push({
        label: label || `rule_${rules.length + 1}`,
        conditions: parseConditionText(condition),
        pricePerCall: parsed.price,
        multiplier: parsed.multiplier,
      })
      const rest = s.slice(bodyEnd + 1).trim()
      if (!rest.startsWith(':')) return
      parseRuleChain(rest.slice(1).trim())
    }

    const chainPrefix = 'v2: '
    if (trimmed.startsWith(chainPrefix) && !body.startsWith('tier("result"')) {
      parseRuleChain(body)
      if (rules.length === 0) return null
      return { base: 'per_call', rules, fallbackPrice, fallbackMultiplier }
    }

    // Backward-compatible old form: tier("result", max(...)).
    // Keep parsing it for existing saved expressions; new saves use the
    // conditional chain above so fallback is only evaluated when needed.
    const outerRe = /^tier\("result",\s*([\s\S]+)\)$/
    const om = body.match(outerRe)
    if (!om) return null

    function splitTopLevelComma(s: string): [string, string] | null {
      let depth = 0
      for (let i = 0; i < s.length; i++) {
        if (s[i] === '(') depth++
        else if (s[i] === ')') depth--
        else if (s[i] === ',' && depth === 0) {
          return [s.slice(0, i).trim(), s.slice(i + 1).trim()]
        }
      }
      return null
    }

    function parseLegacyMax(s: string): void {
      if (!s.startsWith('max(') || !s.endsWith(')')) return
      const pair = splitTopLevelComma(s.slice(4, -1).trim())
      if (!pair) return
      const [term, rest] = pair
      const conditional = term.match(/^([\s\S]+?)\s*\?\s*([\s\S]+?)\s*:\s*0\.0$/)
      if (conditional) {
        const parsed = parsePriceAndMul(conditional[2])
        if (parsed) {
          rules.push({
            label: `rule_${rules.length + 1}`,
            conditions: parseConditionText(conditional[1].trim()),
            pricePerCall: parsed.price,
            multiplier: parsed.multiplier,
          })
        }
      }
      if (rest.startsWith('max(')) {
        parseLegacyMax(rest)
      } else {
        const parsed = parsePriceAndMul(rest)
        if (parsed) {
          fallbackPrice = parsed.price
          fallbackMultiplier = parsed.multiplier
        }
      }
    }

    parseLegacyMax(om[1].trim())
    // The legacy expression uses nested max() and leaves the fallback as the
    // final numeric argument. The recursive parser above handles normal
    // nesting; this fallback extraction also handles whitespace/parenthesis
    // variations emitted by older editor versions.
    if (fallbackPrice === 0) {
      const fallbackMatch = om[1].match(/,\s*(-?[\d.eE+]+)\s*\)\s*\)$/)
      const parsed = fallbackMatch ? parsePriceAndMul(fallbackMatch[1]) : null
      if (parsed) {
        fallbackPrice = parsed.price
        fallbackMultiplier = parsed.multiplier
      }
    }
    if (rules.length === 0) return null
    return { base: 'per_call', rules, fallbackPrice, fallbackMultiplier }
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
      isnull: (val: unknown, fallback: number) => val == null ? fallback : Number(val),
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
