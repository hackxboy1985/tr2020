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

import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Input,
  Select,
  InputNumber,
  Tabs,
  TabPane,
  Banner,
  Typography,
  Space,
  Tooltip,
  Divider,
  TextArea,
  Switch,
} from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconDelete,
  IconAlertTriangle,
  IconInfoCircle,
} from '@douyinfe/semi-icons';

const { Text } = Typography;

// 唯一ID生成器
const generateId = (() => {
  let counter = 0;
  return () => `rule_${counter++}`;
})();

const OPERATOR_OPTIONS = [
  { value: '>', label: '> 大于' },
  { value: '>=', label: '>= 大于等于' },
  { value: '<', label: '< 小于' },
  { value: '<=', label: '<= 小于等于' },
  { value: '==', label: '== 等于' },
  { value: '!=', label: '!= 不等于' },
  { value: 'in', label: 'in 在数组中' },
  { value: 'not_in', label: 'not_in 不在数组中' },
];

const PARAM_OPTIONS = [
  { value: 'temperature', label: 'temperature - 温度' },
  { value: 'max_tokens', label: 'max_tokens - 最大Token' },
  { value: 'max_completion_tokens', label: 'max_completion_tokens - 最大完成Token' },
  { value: 'top_p', label: 'top_p - Top-P采样' },
  { value: 'top_k', label: 'top_k - Top-K采样' },
  { value: 'stream', label: 'stream - 流式输出' },
  { value: 'frequency_penalty', label: 'frequency_penalty - 频率惩罚' },
  { value: 'presence_penalty', label: 'presence_penalty - 存在惩罚' },
  { value: 'n', label: 'n - 生成数量' },
  { value: 'reasoning_effort', label: 'reasoning_effort - 推理强度' },
];

const ModelMappingV2Editor = ({ value = '', onChange }) => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState('v2');
  const [rules, setRules] = useState([]);
  const [v1Mappings, setV1Mappings] = useState([]);
  const [jsonText, setJsonText] = useState('');
  const [parseError, setParseError] = useState('');

  // 初始化
  useEffect(() => {
    if (!value || typeof value !== 'string') {
      setRules([]);
      setV1Mappings([]);
      setJsonText('');
      return;
    }

    try {
      const parsed = JSON.parse(value);

      // 检测是否为V2格式
      if (parsed.version === 2) {
        // 解析规则
        const loadedRules = (parsed.rules || []).map(rule => ({
          id: generateId(),
          source_model: rule.source_model || '',
          target_model: rule.target_model || '',
          priority: rule.priority || 0,
          description: rule.description || '',
          conditions: (rule.conditions || []).map(cond => ({
            id: generateId(),
            param: cond.param || '',
            operator: cond.operator || '>',
            value: cond.value !== undefined ? cond.value : '',
          })),
        }));
        setRules(loadedRules);

        // 解析V1兼容字段
        const v1Compat = [];
        Object.keys(parsed).forEach(key => {
          if (key !== 'version' && key !== 'rules' && typeof parsed[key] === 'string') {
            v1Compat.push({
              id: generateId(),
              source: key,
              target: parsed[key],
            });
          }
        });
        setV1Mappings(v1Compat);
      } else {
        // V1格式，转换为V1兼容映射
        const v1Compat = Object.entries(parsed).map(([key, val]) => ({
          id: generateId(),
          source: key,
          target: val,
        }));
        setV1Mappings(v1Compat);
        setRules([]);
      }

      setJsonText(JSON.stringify(parsed, null, 2));
      setParseError('');
    } catch (e) {
      setParseError(e.message);
      setJsonText(value);
    }
  }, [value]);

  // 生成JSON配置
  const generateConfig = useCallback(() => {
    const config = {
      version: 2,
      rules: rules.map(rule => {
        const ruleObj = {
          source_model: rule.source_model,
          target_model: rule.target_model,
        };

        if (rule.conditions && rule.conditions.length > 0) {
          ruleObj.conditions = rule.conditions
            .filter(cond => cond.param && cond.operator)
            .map(cond => ({
              param: cond.param,
              operator: cond.operator,
              value: cond.value,
            }));
        }

        if (rule.priority) {
          ruleObj.priority = rule.priority;
        }

        if (rule.description) {
          ruleObj.description = rule.description;
        }

        return ruleObj;
      }).filter(rule => rule.source_model && rule.target_model),
    };

    // 添加V1兼容字段
    v1Mappings.forEach(mapping => {
      if (mapping.source && mapping.target) {
        config[mapping.source] = mapping.target;
      }
    });

    return config;
  }, [rules, v1Mappings]);

  // 保存配置
  const saveConfig = useCallback(() => {
    const config = generateConfig();
    const jsonStr = JSON.stringify(config);
    onChange?.(jsonStr);
  }, [generateConfig, onChange]);

  // 规则操作
  const addRule = () => {
    setRules([...rules, {
      id: generateId(),
      source_model: '',
      target_model: '',
      priority: 0,
      description: '',
      conditions: [],
    }]);
  };

  const updateRule = (id, field, value) => {
    setRules(rules.map(rule =>
      rule.id === id ? { ...rule, [field]: value } : rule
    ));
  };

  const deleteRule = (id) => {
    setRules(rules.filter(rule => rule.id !== id));
  };

  const addCondition = (ruleId) => {
    setRules(rules.map(rule =>
      rule.id === ruleId
        ? {
            ...rule,
            conditions: [
              ...rule.conditions,
              { id: generateId(), param: 'temperature', operator: '>', value: '' }
            ]
          }
        : rule
    ));
  };

  const updateCondition = (ruleId, condId, field, value) => {
    setRules(rules.map(rule =>
      rule.id === ruleId
        ? {
            ...rule,
            conditions: rule.conditions.map(cond =>
              cond.id === condId ? { ...cond, [field]: value } : cond
            )
          }
        : rule
    ));
  };

  const deleteCondition = (ruleId, condId) => {
    setRules(rules.map(rule =>
      rule.id === ruleId
        ? { ...rule, conditions: rule.conditions.filter(cond => cond.id !== condId) }
        : rule
    ));
  };

  // V1映射操作
  const addV1Mapping = () => {
    setV1Mappings([...v1Mappings, { id: generateId(), source: '', target: '' }]);
  };

  const updateV1Mapping = (id, field, value) => {
    setV1Mappings(v1Mappings.map(mapping =>
      mapping.id === id ? { ...mapping, [field]: value } : mapping
    ));
  };

  const deleteV1Mapping = (id) => {
    setV1Mappings(v1Mappings.filter(mapping => mapping.id !== id));
  };

  // JSON模式保存
  const saveJsonText = () => {
    try {
      const parsed = JSON.parse(jsonText);
      const jsonStr = JSON.stringify(parsed);
      onChange?.(jsonStr);
      setParseError('');
    } catch (e) {
      setParseError(e.message);
    }
  };

  return (
    <div className="model-mapping-v2-editor">
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        type="line"
      >
        <TabPane
          tab={
            <span>
              <Text>表单模式</Text>
            </span>
          }
          itemKey="v2"
        >
          <div className="space-y-4">
            <Banner
              type="info"
              icon={<IconInfoCircle />}
              description="V2版本支持基于请求参数的条件映射。配置会自动兼容旧版本服务器。"
            />

            {/* V2规则 */}
            <Card
              title={
                <div className="flex items-center justify-between">
                  <Text strong>条件映射规则（V2）</Text>
                  <Button
                    icon={<IconPlus />}
                    onClick={addRule}
                    size="small"
                  >
                    添加规则
                  </Button>
                </div>
              }
              bordered
            >
              {rules.length === 0 ? (
                <div className="text-center text-gray-400 py-6">
                  暂无规则，点击"添加规则"开始配置
                </div>
              ) : (
                <Space vertical style={{ width: '100%' }} spacing={16}>
                  {rules.map((rule, ruleIndex) => (
                    <Card
                      key={rule.id}
                      bordered
                      bodyStyle={{ padding: '12px' }}
                      headerStyle={{ padding: '8px 12px' }}
                      title={
                        <Text size="small" type="tertiary">
                          规则 #{ruleIndex + 1}
                        </Text>
                      }
                      headerExtraContent={
                        <Button
                          icon={<IconDelete />}
                          type="danger"
                          theme="borderless"
                          size="small"
                          onClick={() => deleteRule(rule.id)}
                        />
                      }
                    >
                      <Space vertical style={{ width: '100%' }} spacing={8}>
                        {/* 源模型 → 目标模型 */}
                        <div className="flex items-center gap-2">
                          <Input
                            placeholder="源模型（如 gpt-4）"
                            value={rule.source_model}
                            onChange={(val) => updateRule(rule.id, 'source_model', val)}
                            style={{ flex: 1 }}
                            size="small"
                          />
                          <Text type="tertiary">→</Text>
                          <Input
                            placeholder="目标模型（如 gpt-4-turbo）"
                            value={rule.target_model}
                            onChange={(val) => updateRule(rule.id, 'target_model', val)}
                            style={{ flex: 1 }}
                            size="small"
                          />
                        </div>

                        {/* 条件 */}
                        <div>
                          <div className="flex items-center justify-between mb-2">
                            <Text size="small" type="tertiary">条件（所有条件需同时满足）</Text>
                            <Button
                              icon={<IconPlus />}
                              size="small"
                              type="tertiary"
                              onClick={() => addCondition(rule.id)}
                            >
                              添加条件
                            </Button>
                          </div>
                          {rule.conditions.length === 0 ? (
                            <Text size="small" type="quaternary">
                              无条件（直接匹配）
                            </Text>
                          ) : (
                            <Space vertical style={{ width: '100%' }} spacing={4}>
                              {rule.conditions.map((cond) => (
                                <div key={cond.id} className="flex items-center gap-2">
                                  <Select
                                    value={cond.param}
                                    onChange={(val) => updateCondition(rule.id, cond.id, 'param', val)}
                                    optionList={PARAM_OPTIONS}
                                    style={{ width: 200 }}
                                    size="small"
                                    filter
                                  />
                                  <Select
                                    value={cond.operator}
                                    onChange={(val) => updateCondition(rule.id, cond.id, 'operator', val)}
                                    optionList={OPERATOR_OPTIONS}
                                    style={{ width: 120 }}
                                    size="small"
                                  />
                                  <Input
                                    value={cond.value}
                                    onChange={(val) => updateCondition(rule.id, cond.id, 'value', val)}
                                    placeholder="值"
                                    style={{ flex: 1 }}
                                    size="small"
                                  />
                                  <Button
                                    icon={<IconDelete />}
                                    type="danger"
                                    theme="borderless"
                                    size="small"
                                    onClick={() => deleteCondition(rule.id, cond.id)}
                                  />
                                </div>
                              ))}
                            </Space>
                          )}
                        </div>

                        {/* 优先级和描述 */}
                        <div className="flex gap-2">
                          <InputNumber
                            prefix="优先级"
                            value={rule.priority}
                            onChange={(val) => updateRule(rule.id, 'priority', val)}
                            placeholder="0"
                            style={{ width: 120 }}
                            size="small"
                          />
                          <Input
                            prefix="描述"
                            value={rule.description}
                            onChange={(val) => updateRule(rule.id, 'description', val)}
                            placeholder="可选"
                            style={{ flex: 1 }}
                            size="small"
                          />
                        </div>
                      </Space>
                    </Card>
                  ))}
                </Space>
              )}
            </Card>

            {/* V1兼容映射 */}
            <Card
              title={
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Text strong>简单映射（V1兼容）</Text>
                    <Tooltip content="用于旧版本服务器和无条件映射">
                      <IconInfoCircle size="small" />
                    </Tooltip>
                  </div>
                  <Button
                    icon={<IconPlus />}
                    onClick={addV1Mapping}
                    size="small"
                  >
                    添加映射
                  </Button>
                </div>
              }
              bordered
            >
              {v1Mappings.length === 0 ? (
                <div className="text-center text-gray-400 py-6">
                  暂无映射
                </div>
              ) : (
                <Space vertical style={{ width: '100%' }} spacing={8}>
                  {v1Mappings.map((mapping) => (
                    <div key={mapping.id} className="flex items-center gap-2">
                      <Input
                        placeholder="源模型"
                        value={mapping.source}
                        onChange={(val) => updateV1Mapping(mapping.id, 'source', val)}
                        style={{ flex: 1 }}
                        size="small"
                      />
                      <Text type="tertiary">→</Text>
                      <Input
                        placeholder="目标模型"
                        value={mapping.target}
                        onChange={(val) => updateV1Mapping(mapping.id, 'target', val)}
                        style={{ flex: 1 }}
                        size="small"
                      />
                      <Button
                        icon={<IconDelete />}
                        type="danger"
                        theme="borderless"
                        size="small"
                        onClick={() => deleteV1Mapping(mapping.id)}
                      />
                    </div>
                  ))}
                </Space>
              )}
            </Card>

            <div className="flex justify-end">
              <Button type="primary" onClick={saveConfig}>
                保存配置
              </Button>
            </div>
          </div>
        </TabPane>

        <TabPane
          tab={<Text>JSON模式</Text>}
          itemKey="json"
        >
          <div className="space-y-4">
            {parseError && (
              <Banner
                type="danger"
                icon={<IconAlertTriangle />}
                description={`JSON解析错误: ${parseError}`}
              />
            )}
            <TextArea
              value={jsonText}
              onChange={setJsonText}
              rows={20}
              placeholder="请输入JSON配置"
              style={{ fontFamily: 'monospace' }}
            />
            <div className="flex justify-end">
              <Button type="primary" onClick={saveJsonText}>
                保存配置
              </Button>
            </div>
          </div>
        </TabPane>
      </Tabs>
    </div>
  );
};

export default ModelMappingV2Editor;
