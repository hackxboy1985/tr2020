/*
Copyright (C) 2025 QuantumNous

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

import React, { useEffect, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Col,
  Form,
  Row,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  compareObjects,
  parseHttpStatusCodeRules,
  showError,
  showSuccess,
  showWarning,
  toBoolean,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import HttpStatusCodeRulesInput from '../../../components/settings/HttpStatusCodeRulesInput';

const KEY_ENABLED = 'overload_sleep_setting.enabled';
const KEY_SLEEP_MINUTES = 'overload_sleep_setting.sleep_minutes';
const KEY_STATUS_CODES = 'overload_sleep_setting.status_codes';
const KEY_FALLBACK_MODELS = 'overload_sleep_setting.fallback_models';

const DEFAULT_INPUTS = {
  [KEY_ENABLED]: false,
  [KEY_SLEEP_MINUTES]: 2,
  [KEY_STATUS_CODES]: '429,503',
  [KEY_FALLBACK_MODELS]: '',
};

const MIN_SLEEP_MINUTES = 1;
const MAX_SLEEP_MINUTES = 1440;

export default function SettingsOverloadSleep(props) {
  const { t } = useTranslation();
  const { Text } = Typography;
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({ ...DEFAULT_INPUTS });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState({ ...DEFAULT_INPUTS });
  const parsedStatusCodes = parseHttpStatusCodeRules(
    inputs[KEY_STATUS_CODES] || '',
  );

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((current) => ({ ...current, [fieldName]: value }));
    };
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));

    if (!parsedStatusCodes.ok) {
      const details =
        parsedStatusCodes.invalidTokens &&
        parsedStatusCodes.invalidTokens.length > 0
          ? `: ${parsedStatusCodes.invalidTokens.join(', ')}`
          : '';
      return showError(`${t('过载状态码格式不正确')}${details}`);
    }
    const sleepMinutes = Number(inputs[KEY_SLEEP_MINUTES]);
    if (
      !Number.isFinite(sleepMinutes) ||
      sleepMinutes < MIN_SLEEP_MINUTES ||
      sleepMinutes > MAX_SLEEP_MINUTES
    ) {
      return showError(t('休眠时长需在 1 - 1440 分钟之间'));
    }

    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (item.key === KEY_STATUS_CODES) {
        value = parsedStatusCodes.normalized;
      } else if (item.key === KEY_SLEEP_MINUTES) {
        value = String(Math.round(sleepMinutes));
      } else {
        value = String(inputs[item.key]);
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });

    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined)) {
            return showError(t('部分保存失败，请重试'));
          }
        }
        showSuccess(t('保存成功'));
        if (props.refresh) {
          props.refresh();
        }
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = { ...DEFAULT_INPUTS };
    for (const key in props.options) {
      if (!Object.prototype.hasOwnProperty.call(DEFAULT_INPUTS, key)) continue;
      if (key === KEY_ENABLED) {
        currentInputs[key] = toBoolean(props.options[key]);
      } else if (key === KEY_SLEEP_MINUTES) {
        const minutes = Number(props.options[key]);
        currentInputs[key] =
          Number.isFinite(minutes) && minutes > 0
            ? minutes
            : DEFAULT_INPUTS[KEY_SLEEP_MINUTES];
      } else {
        currentInputs[key] =
          props.options[key] === undefined || props.options[key] === null
            ? DEFAULT_INPUTS[key]
            : String(props.options[key]);
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    if (refForm.current) {
      refForm.current.setValues(currentInputs);
    }
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('过载休眠池')}>
            <Banner
              fullMode={false}
              type='info'
              description={t(
                '开启后，上游返回配置的过载状态码时，会把对应的「模型 + 渠道」加入休眠池，后续请求会跳过该渠道；当某模型的全部渠道都在休眠时，会取最后一个渠道强行尝试。',
              )}
            />
            <Row gutter={16} style={{ marginTop: 12 }}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={KEY_ENABLED}
                  label={t('启用过载休眠池')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(KEY_ENABLED)}
                />
                <Text type='tertiary' size='small'>
                  {t(
                    '关闭时整套流程不生效：不写入休眠池、不跳过渠道，也不写跳过日志。',
                  )}
                </Text>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={KEY_SLEEP_MINUTES}
                  label={t('休眠时长')}
                  step={1}
                  min={MIN_SLEEP_MINUTES}
                  max={MAX_SLEEP_MINUTES}
                  suffix={t('分钟')}
                  extraText={t(
                    '命中过载状态码后，该「模型 + 渠道」在休眠池中保留的时长（1-1440 分钟）。',
                  )}
                  placeholder={''}
                  onChange={handleFieldChange(KEY_SLEEP_MINUTES)}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <HttpStatusCodeRulesInput
                  label={t('过载状态码')}
                  placeholder={t('例如：429, 503, 500-599')}
                  extraText={t(
                    '支持填写单个状态码或范围（含首尾），使用逗号分隔，例如：429, 503, 500-599',
                  )}
                  field={KEY_STATUS_CODES}
                  onChange={handleFieldChange(KEY_STATUS_CODES)}
                  parsed={parsedStatusCodes}
                  invalidText={t('过载状态码格式不正确')}
                />
              </Col>
            </Row>
            <Row gutter={16} style={{ marginTop: 12 }}>
              <Col xs={24} sm={16}>
                <Form.TextArea
                  field={KEY_FALLBACK_MODELS}
                  label={t('备用模型')}
                  placeholder={'gemini-3.6-flash=gemini-3.7-flash'}
                  extraText={t(
                    '每行一个映射，格式：原模型=备用模型。仅当原模型全部渠道休眠、且 API 密钥开启了「使用相近模型兜底」时生效。',
                  )}
                  autosize={{ minRows: 3, maxRows: 12 }}
                  onChange={handleFieldChange(KEY_FALLBACK_MODELS)}
                />
              </Col>
            </Row>
            <Row style={{ marginTop: 12 }}>
              <Button size='default' onClick={onSubmit}>
                {t('保存过载休眠设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
