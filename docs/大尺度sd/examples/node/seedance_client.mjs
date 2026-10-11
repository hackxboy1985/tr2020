// Customer-gateway example. Default action is read-only; --run creates a paid task.
import { createWriteStream } from 'node:fs';
import { randomUUID } from 'node:crypto';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';

const options = Object.fromEntries(process.argv.slice(2).filter(x => x.startsWith('--') && x.includes('='))
  .map(x => { const at = x.indexOf('='); return [x.slice(2, at), x.slice(at + 1)]; }));
const run = process.argv.includes('--run');
if (process.argv.includes('--help')) {
  console.log('Set SEEDANCE_BASE_URL and SEEDANCE_API_KEY. Use --run to create a paid task. Options: --model=ID --prompt=TEXT --seconds=5 --reference-url=URL --asset-uri=asset://task_... --reference-type=image|video|audio --idempotency-key=KEY --output=FILE');
  process.exit(0);
}
const base = (process.env.SEEDANCE_BASE_URL || '').replace(/\/+$/, '');
const key = process.env.SEEDANCE_API_KEY || '';
if (!/^https?:\/\//.test(base) || !key) throw new Error('Set SEEDANCE_BASE_URL and SEEDANCE_API_KEY');

async function api(method, path, payload, idem) {
  const headers = { Authorization: `Bearer ${key}` };
  if (payload !== undefined) headers['Content-Type'] = 'application/json';
  if (idem) headers['Idempotency-Key'] = idem;
  let response;
  try {
    response = await fetch(base + path, { method, headers, body: payload === undefined ? undefined : JSON.stringify(payload), signal: AbortSignal.timeout(210000) });
  } catch (error) {
    throw new Error(`Network error; request outcome may be unknown: ${error.message}`);
  }
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(`HTTP ${response.status}: ${body.error?.code || 'unknown_error'}: ${body.error?.message || ''}`);
  return body;
}

async function poll(method, path, payload, done, failures, everyMs, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (true) {
    const item = await api(method, path, payload);
    console.log(`${item.id || '?'}: ${item.status}`);
    if (item.status === done) return item;
    if (failures.includes(item.status)) throw new Error(`Task ended as ${item.status}: ${JSON.stringify(item.error || item)}`);
    if (Date.now() >= deadline) throw new Error(`Polling timed out; keep task ID ${item.id} for later query`);
    await new Promise(resolve => setTimeout(resolve, everyMs));
  }
}

async function main() {
  const models = await api('GET', '/v1/models');
  const ids = new Set((models.data || []).map(item => item.id));
  console.log('Visible models:', [...ids].sort().join(', '));
  if (!run) return console.log('Read-only check complete. Pass --run to create a billable task.');
  const model = options.model || 'seedance-2.0-720p';
  if (!ids.has(model)) throw new Error(`Model ${model} is not visible to this API Key`);
  const prompt = options.prompt || '古代街道，镜头缓慢向前推进';
  const seconds = Number(options.seconds || 5);
  if (!Number.isInteger(seconds) || seconds < 1 || seconds > 30) throw new Error('--seconds must be an integer from 1 to 30');
  const type = options['reference-type'] || 'video';
  if (!['image', 'video', 'audio'].includes(type)) throw new Error('--reference-type must be image, video or audio');

  if (options['reference-url'] && options['asset-uri']) throw new Error('Choose --reference-url or --asset-uri, not both');
  let assetUri = options['asset-uri'];
  if (assetUri && (!assetUri.startsWith('asset://task_') || !/^[A-Za-z0-9_-]+$/.test(assetUri.slice(13)))) throw new Error('--asset-uri must be asset://task_...');
  if ((options['reference-url'] || assetUri) && !ids.has('media-service')) throw new Error('media-service is required for reference material');
  if (options['reference-url']) {
    if (!/^https?:\/\//.test(options['reference-url'])) throw new Error('--reference-url must be a raw HTTP(S) URL');
    const imported = await api('POST', '/seedance/media', { action: 'import', url: options['reference-url'], media_type: type, title: 'API example reference' });
    console.log('Asset ID:', imported.id);
    assetUri = imported.asset_uri;
  }
  if (assetUri) {
    console.log('Asset URI:', assetUri);
    const asset = await poll('POST', '/seedance/media', { action: 'get', id: assetUri.slice(8) }, 'succeeded', ['failed', 'deleted'], 2000, 300000);
    const preflight = await api('POST', '/seedance/media/preflight', { asset_id: asset.id, model, media_type: type, role: `reference_${type}` });
    console.log('Preflight:', preflight.compatibility);
    if (preflight.compatibility === 'unsupported') throw new Error('Known incompatible reference; no billable task submitted');
  }

  let payload;
  if (assetUri && type !== 'image') {
    const kind = `${type}_url`;
    payload = { model, duration: seconds, ratio: '16:9', n: 1,
      content: [{ type: 'text', text: prompt }, { type: kind, [kind]: { url: assetUri }, role: `reference_${type}` }] };
  } else {
    payload = { model, prompt, seconds };
    if (assetUri) payload.input_reference = assetUri;
  }
  const idem = options['idempotency-key'] || `example-${randomUUID()}`;
  console.log('Idempotency-Key:', idem);
  console.log('Submitting one billable video request.');
  const created = await api('POST', '/v1/videos', payload, idem);
  console.log('Video task ID:', created.id);
  const video = await poll('GET', `/v1/videos/${encodeURIComponent(created.id)}`, undefined, 'completed', ['failed'], 10000, 1200000);
  const path = video.url || video.urls?.[0];
  if (!path || !/^\/v1\/(videos|tasks)\//.test(path) || path.includes('://')) throw new Error('Completed task has no valid download path');
  const response = await fetch(base + path, { headers: { Authorization: `Bearer ${key}` }, signal: AbortSignal.timeout(120000) });
  if (!response.ok) throw new Error(`Download HTTP ${response.status}`);
  const output = options.output || 'seedance-output.mp4';
  await pipeline(Readable.fromWeb(response.body), createWriteStream(output));
  console.log('Saved:', output);
}

main().catch(error => { console.error('Error:', error.message); process.exitCode = 1; });
