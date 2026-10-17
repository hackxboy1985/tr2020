import React from 'react';
import { Banner, Switch, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

export function parseZyOssConfig(value) {
  if (!value?.trim()) return {};
  const parsed = JSON.parse(value);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Invalid JSON format');
  }
  return parsed;
}

export default function ZyOssConfig(props) {
  const { t } = useTranslation();
  let config = {};
  let valid = true;
  try {
    config = parseZyOssConfig(props.value);
  } catch {
    valid = false;
  }

  function update(key, checked) {
    if (!valid) return;
    props.onChange(JSON.stringify({ ...config, [key]: checked }, null, 2));
  }

  return (
    <div className='my-4 rounded-lg border p-4 space-y-3'>
      <Typography.Text strong>{t('Object Storage Transfer')}</Typography.Text>
      {!valid && (
        <Banner type='danger' description={t('Invalid JSON format')} />
      )}
      {[
        ['zy_oss_image_enabled', 'Transfer image results to object storage'],
        ['zy_oss_video_enabled', 'Transfer video results to object storage'],
      ].map(([key, label]) => (
        <div key={key} className='flex items-center justify-between gap-4'>
          <label htmlFor={key}>{t(label)}</label>
          <Switch
            id={key}
            checked={config[key] === true}
            disabled={!valid}
            onChange={(checked) => update(key, checked)}
          />
        </div>
      ))}
      <Typography.Paragraph type='tertiary' size='small'>
        {t(
          'Uses the shared S3 configuration. Successful transfers return the storage URL; failed transfers fall back to the upstream URL.',
        )}
      </Typography.Paragraph>
    </div>
  );
}
