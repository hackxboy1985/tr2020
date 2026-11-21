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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Empty,
  Input,
  Modal,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { IconDelete, IconRefresh, IconSearch } from '@douyinfe/semi-icons';
import { API, showError, showSuccess, showWarning } from '../../helpers';
import { CHANNEL_OPTIONS } from '../../constants';

const CHANNEL_TYPE_MAP = CHANNEL_OPTIONS.reduce((acc, option) => {
  acc[option.value] = option;
  return acc;
}, {});

const getChannelTypeMeta = (channelType) => {
  const meta = CHANNEL_TYPE_MAP[channelType];
  if (!meta) {
    return { label: `#${channelType}`, color: 'grey' };
  }
  return { label: meta.label, color: meta.color };
};

const getEntryKey = (entry) =>
  `${entry?.model_name ?? ''}::${entry?.channel_id ?? ''}`;

const formatChannelName = (name) => {
  if (!name) return '-';
  return name;
};

const OverloadSleep = () => {
  const { t } = useTranslation();
  const { Text } = Typography;

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [data, setData] = useState({
    enabled: false,
    sleep_minutes: 0,
    status_codes: '',
    redis_enabled: false,
    server_time: 0,
    entries: [],
  });
  const [fetchedAt, setFetchedAt] = useState(0);
  const [now, setNow] = useState(() => Date.now());
  const [modelFilter, setModelFilter] = useState('');
  const [channelFilter, setChannelFilter] = useState('');
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [removing, setRemoving] = useState(false);

  // 每秒刷新一次本地时间，用于剩余秒数倒计时
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);

  const serverOffset = useMemo(() => {
    const serverTime = Number(data?.server_time) || 0;
    if (!serverTime || !fetchedAt) return 0;
    return serverTime * 1000 - fetchedAt;
  }, [data?.server_time, fetchedAt]);

  const loadSleepPool = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/channel_sleep_pool/', {
        disableDuplicate: true,
      });
      const { success, message, data: payload } = res.data || {};
      if (!success) {
        setError(message || t('获取休眠池失败'));
        showError(message || t('获取休眠池失败'));
        return;
      }
      setData({
        enabled: payload?.enabled === true,
        sleep_minutes: Number(payload?.sleep_minutes) || 0,
        status_codes: payload?.status_codes ?? '',
        redis_enabled: payload?.redis_enabled === true,
        server_time: Number(payload?.server_time) || 0,
        entries: Array.isArray(payload?.entries) ? payload.entries : [],
      });
      setFetchedAt(Date.now());
      setSelectedRowKeys([]);
    } catch (err) {
      setError(t('获取休眠池失败'));
      showError(t('获取休眠池失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadSleepPool();
  }, [loadSleepPool]);

  const remainingSeconds = useCallback(
    (entry) => {
      const sleepUntil = Number(entry?.sleep_until) || 0;
      if (sleepUntil > 0) {
        const serverNow = now + serverOffset;
        return Math.max(0, Math.ceil(sleepUntil - serverNow / 1000));
      }
      const raw = Number(entry?.remaining_seconds) || 0;
      const elapsed = fetchedAt ? Math.floor((now - fetchedAt) / 1000) : 0;
      return Math.max(0, raw - elapsed);
    },
    [now, serverOffset, fetchedAt],
  );

  const filteredEntries = useMemo(() => {
    const modelKeyword = modelFilter.trim().toLowerCase();
    const channelKeyword = channelFilter.trim().toLowerCase();
    return (data.entries || []).filter((entry) => {
      const modelName = (entry?.model_name || '').toLowerCase();
      if (modelKeyword && !modelName.includes(modelKeyword)) return false;
      if (!channelKeyword) return true;
      const channelName = (entry?.channel_name || '').toLowerCase();
      const channelId = String(entry?.channel_id ?? '').toLowerCase();
      return (
        channelName.includes(channelKeyword) ||
        channelId.includes(channelKeyword)
      );
    });
  }, [data.entries, modelFilter, channelFilter]);

  const removeEntries = async (entries) => {
    if (!entries || entries.length === 0) {
      return showWarning(t('请先选择要移除的记录'));
    }
    setRemoving(true);
    try {
      const queue = entries.map((entry) =>
        API.delete('/api/channel_sleep_pool/', {
          params: {
            model_name: entry.model_name,
            channel_id: entry.channel_id,
          },
        }),
      );
      const results = await Promise.all(queue);
      const failed = results.filter(
        (res) => res?.data && res.data.success === false,
      );
      if (failed.length > 0) {
        showError(failed[0]?.data?.message || t('移除失败，请重试'));
      } else {
        showSuccess(
          entries.length > 1
            ? t('已移除 {{total}} 条休眠记录', { total: entries.length })
            : t('已移除该条休眠记录'),
        );
      }
      await loadSleepPool();
    } catch (err) {
      showError(t('移除失败，请重试'));
    } finally {
      setRemoving(false);
    }
  };

  const confirmRemoveEntry = (entry) => {
    Modal.confirm({
      title: t('确认从休眠池移除'),
      content: (
        <div style={{ lineHeight: '1.7' }}>
          <Text>
            {t('将把模型「{{model}}」的渠道 #{{id}} 从休眠池中移除。', {
              model: entry?.model_name ?? '-',
              id: entry?.channel_id ?? '-',
            })}
          </Text>
        </div>
      ),
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: () => removeEntries([entry]),
    });
  };

  const confirmRemoveSelected = () => {
    const entries = filteredEntries.filter((entry) =>
      selectedRowKeys.includes(getEntryKey(entry)),
    );
    if (entries.length === 0) {
      return showWarning(t('请先选择要移除的记录'));
    }
    Modal.confirm({
      title: t('确认从休眠池移除'),
      content: (
        <div style={{ lineHeight: '1.7' }}>
          <Text>
            {t('将从休眠池中移除选中的 {{total}} 条记录。', {
              total: entries.length,
            })}
          </Text>
        </div>
      ),
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: () => removeEntries(entries),
    });
  };

  const confirmClearAll = () => {
    Modal.confirm({
      title: t('确认全部清除'),
      content: (
        <div style={{ lineHeight: '1.7' }}>
          <Text>
            {t('将清空所有模型的休眠池记录，此操作不可撤销，确定继续吗？')}
          </Text>
        </div>
      ),
      okText: t('确定'),
      cancelText: t('取消'),
      onOk: async () => {
        setRemoving(true);
        try {
          const res = await API.delete('/api/channel_sleep_pool/', {
            params: { all: true },
          });
          const { success, message, data: payload } = res.data || {};
          if (!success) {
            showError(message || t('全部清除失败，请重试'));
          } else {
            showSuccess(
              t('已清除 {{total}} 条休眠记录', {
                total: Number(payload?.removed) || 0,
              }),
            );
          }
          await loadSleepPool();
        } catch (err) {
          showError(t('全部清除失败，请重试'));
        } finally {
          setRemoving(false);
        }
      },
    });
  };

  const resetFilters = () => {
    setModelFilter('');
    setChannelFilter('');
  };

  const columns = useMemo(
    () => [
      {
        title: t('模型名'),
        dataIndex: 'model_name',
        width: 200,
        render: (text) => <Text strong>{text || '-'}</Text>,
      },
      {
        title: t('渠道'),
        width: 220,
        render: (_, record) => (
          <Space spacing={4}>
            <Tag color='blue'>#{record?.channel_id ?? '-'}</Tag>
            <Text>{formatChannelName(record?.channel_name)}</Text>
          </Space>
        ),
      },
      {
        title: t('渠道类型'),
        dataIndex: 'channel_type',
        width: 160,
        render: (channelType) => {
          const meta = getChannelTypeMeta(channelType);
          return <Tag color={meta.color}>{meta.label}</Tag>;
        },
      },
      {
        title: t('触发状态码'),
        dataIndex: 'trigger_status_code',
        width: 110,
        render: (code) =>
          code === undefined || code === null || code === 0 ? (
            <Text type='tertiary'>-</Text>
          ) : (
            <Tag color='orange'>{String(code)}</Tag>
          ),
      },
      {
        title: t('触发次数'),
        dataIndex: 'trigger_count',
        width: 100,
        render: (count) => <Text>{Number(count) || 0}</Text>,
      },
      {
        title: t('剩余秒数'),
        width: 120,
        render: (_, record) => {
          const seconds = remainingSeconds(record);
          return (
            <Tag color={seconds > 0 ? 'green' : 'grey'}>
              {seconds > 0 ? t('{{seconds}} 秒', { seconds }) : t('已过期')}
            </Tag>
          );
        },
      },
      {
        title: t('跳过休眠渠道后会走到的渠道'),
        width: 320,
        render: (_, record) => {
          if (record?.all_channels_sleeping) {
            const fallback = record?.fallback_channel;
            return (
              <Space vertical spacing={4} align='start'>
                <Tag color='red'>{t('全部渠道都在休眠')}</Tag>
                {fallback ? (
                  <Text>
                    {t('全部休眠，取最后一个渠道')}：
                    <Text strong>
                      {formatChannelName(fallback?.channel_name)}
                    </Text>{' '}
                    <Tag color='blue'>#{fallback?.channel_id ?? '-'}</Tag>
                  </Text>
                ) : (
                  <Text type='tertiary'>{t('无候选渠道')}</Text>
                )}
              </Space>
            );
          }
          const nextChannels = Array.isArray(record?.next_channels)
            ? record.next_channels
            : [];
          if (nextChannels.length === 0) {
            return <Text type='tertiary'>{t('无候选渠道')}</Text>;
          }
          return (
            <Space spacing={4} style={{ flexWrap: 'wrap' }}>
              {nextChannels.map((channel) => (
                <Tag key={`${channel?.channel_id ?? ''}`} color='blue'>
                  {formatChannelName(channel?.channel_name)} (#
                  {channel?.channel_id ?? '-'})
                </Tag>
              ))}
            </Space>
          );
        },
      },
      {
        title: t('操作'),
        width: 100,
        fixed: 'right',
        render: (_, record) => (
          <Button
            icon={<IconDelete />}
            theme='borderless'
            type='danger'
            size='small'
            title={t('从休眠池移除')}
            aria-label={t('从休眠池移除')}
            onClick={() => confirmRemoveEntry(record)}
          >
            {t('移除')}
          </Button>
        ),
      },
    ],
    [t, remainingSeconds],
  );

  const rowSelection = {
    selectedRowKeys,
    onChange: (keys) => setSelectedRowKeys(keys),
  };

  const renderStatusBar = () => (
    <div
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        gap: 16,
        padding: '12px 16px',
        marginBottom: 12,
        borderRadius: 8,
        background: 'var(--semi-color-fill-0)',
        border: '1px solid var(--semi-color-border)',
      }}
    >
      <Space spacing={4}>
        <Text type='tertiary'>{t('总开关')}</Text>
        <Tag color={data.enabled ? 'green' : 'grey'}>
          {data.enabled ? t('已开启') : t('已关闭')}
        </Tag>
      </Space>
      <Space spacing={4}>
        <Text type='tertiary'>{t('休眠时长')}</Text>
        <Text strong>
          {data.sleep_minutes > 0 ? `${data.sleep_minutes} ` : '-'}
          {data.sleep_minutes > 0 ? t('分钟') : ''}
        </Text>
      </Space>
      <Space spacing={4}>
        <Text type='tertiary'>{t('过载状态码')}</Text>
        <Text strong>{data.status_codes || '-'}</Text>
      </Space>
      <Space spacing={4}>
        <Text type='tertiary'>Redis</Text>
        <Tag color={data.redis_enabled ? 'green' : 'red'}>
          {data.redis_enabled ? t('可用') : t('不可用')}
        </Tag>
      </Space>
      <Text type='tertiary' size='small'>
        {t('以上配置为只读，请在「运营设置 - 过载休眠池」中修改')}
      </Text>
    </div>
  );

  return (
    <div className='mt-[60px] px-2'>
      <Spin spinning={loading}>
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            justifyContent: 'space-between',
            alignItems: 'center',
            gap: 12,
            marginBottom: 12,
          }}
        >
          <Space spacing={8}>
            <Typography.Title heading={5} style={{ margin: 0 }}>
              {t('休眠模型')}
            </Typography.Title>
            <Text type='tertiary' size='small'>
              {t('共 {{total}} 条', { total: data.entries.length })}
            </Text>
          </Space>
          <Space>
            <Button
              icon={<IconRefresh />}
              onClick={loadSleepPool}
              loading={loading}
            >
              {t('刷新')}
            </Button>
            <Button
              type='danger'
              icon={<IconDelete />}
              loading={removing}
              disabled={!data.entries.length}
              onClick={confirmClearAll}
            >
              {t('全部清除')}
            </Button>
          </Space>
        </div>

        {!data.redis_enabled && (
          <Banner
            fullMode={false}
            type='danger'
            closable={false}
            description={t('Redis 不可用，休眠池功能当前不可用')}
            style={{ marginBottom: 12 }}
          />
        )}
        {!data.enabled && (
          <Banner
            fullMode={false}
            type='warning'
            closable={false}
            description={t(
              '总开关处于关闭状态，当前不会写入休眠池，也不会跳过渠道',
            )}
            style={{ marginBottom: 12 }}
          />
        )}
        {error && (
          <Banner
            fullMode={false}
            type='danger'
            closable={false}
            description={error}
            style={{ marginBottom: 12 }}
          />
        )}

        {renderStatusBar()}

        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: 12,
            alignItems: 'center',
            marginBottom: 12,
          }}
        >
          <Input
            prefix={<IconSearch />}
            placeholder={t('按模型名筛选')}
            value={modelFilter}
            onChange={setModelFilter}
            showClear
            style={{ width: 240 }}
          />
          <Input
            prefix={<IconSearch />}
            placeholder={t('按渠道名称或渠道 ID 筛选')}
            value={channelFilter}
            onChange={setChannelFilter}
            showClear
            style={{ width: 240 }}
          />
          <Button onClick={resetFilters}>{t('重置')}</Button>
          <Button
            theme='light'
            type='danger'
            icon={<IconDelete />}
            disabled={selectedRowKeys.length === 0}
            loading={removing}
            onClick={confirmRemoveSelected}
          >
            {t('从休眠池移除')}
            {selectedRowKeys.length > 0 ? ` (${selectedRowKeys.length})` : ''}
          </Button>
        </div>

        <Table
          columns={columns}
          dataSource={filteredEntries}
          rowKey={getEntryKey}
          rowSelection={rowSelection}
          pagination={false}
          size='small'
          scroll={{ x: 'max-content' }}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无休眠记录')}
              style={{ padding: 30 }}
            />
          }
          className='overflow-hidden'
        />
      </Spin>
    </div>
  );
};

export default OverloadSleep;
