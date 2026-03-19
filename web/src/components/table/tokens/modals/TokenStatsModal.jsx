import React, { useState, useEffect, useCallback } from 'react';
import {
  SideSheet,
  DatePicker,
  Spin,
  Tabs,
  TabPane,
  Empty,
  Typography,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { API, showError, renderQuota, modelColorMap, modelToColor } from '../../../../helpers';

const CHART_CONFIG = { mode: 'desktop-browser' };

const TokenStatsModal = ({ visible, onClose, token, t }) => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState([]);
  const [activeTab, setActiveTab] = useState('quota');

  const now = new Date();
  const weekAgo = new Date(now.getTime() - 7 * 24 * 3600 * 1000);
  const [dateRange, setDateRange] = useState([weekAgo, now]);

  const loadData = useCallback(async () => {
    if (!token?.id) return;
    setLoading(true);
    try {
      const startTs = Math.floor(dateRange[0].getTime() / 1000);
      const endTs = Math.floor(dateRange[1].getTime() / 1000);
      const res = await API.get(
        `/api/data/token?token_id=${token.id}&start_timestamp=${startTs}&end_timestamp=${endTs}`,
      );
      const { success, message, data: resData } = res.data;
      if (success) {
        setData(resData || []);
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setLoading(false);
    }
  }, [token?.id, dateRange]);

  useEffect(() => {
    if (visible && token?.id) {
      loadData();
    }
  }, [visible, token?.id, loadData]);

  const formatTime = (ts) => {
    const d = new Date(ts * 1000);
    return `${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')} ${d.getHours().toString().padStart(2, '0')}:00`;
  };

  const buildChartData = (valueField) => {
    if (!data || data.length === 0) return [];
    const sorted = [...data].sort((a, b) => a.created_at - b.created_at);
    return sorted.map((item) => ({
      Time: formatTime(item.created_at),
      Model: item.model_name || 'unknown',
      Value: item[valueField] || 0,
      rawValue: item[valueField] || 0,
    }));
  };

  const getColors = () => {
    const models = new Set((data || []).map((d) => d.model_name));
    const colors = {};
    models.forEach((m) => {
      colors[m] = modelColorMap[m] || modelToColor(m);
    });
    return colors;
  };

  const makeSpec = (title, valueField, tooltipFmt) => {
    const chartData = buildChartData(valueField);
    const colors = getColors();
    return {
      type: 'bar',
      data: [{ id: 'data', values: chartData }],
      xField: 'Time',
      yField: 'Value',
      seriesField: 'Model',
      stack: true,
      legends: { visible: true },
      title: { visible: true, text: title },
      bar: {
        state: {
          hover: { stroke: '#000', lineWidth: 1 },
        },
      },
      tooltip: {
        mark: {
          content: [
            {
              key: (datum) => datum['Model'],
              value: (datum) => tooltipFmt(datum['rawValue']),
            },
          ],
        },
      },
      color: { specified: colors },
    };
  };

  const quotaSpec = makeSpec(
    t('额度消耗'),
    'quota',
    (v) => renderQuota(v, 4),
  );
  const tokenSpec = makeSpec(
    t('Token 消耗'),
    'token_used',
    (v) => v.toLocaleString(),
  );
  const countSpec = makeSpec(
    t('请求次数'),
    'count',
    (v) => v.toLocaleString(),
  );

  const hasData = data && data.length > 0;

  return (
    <SideSheet
      visible={visible}
      onCancel={onClose}
      title={`${t('令牌统计')} - ${token?.name || ''}`}
      width={720}
      placement='right'
    >
      <div className='flex flex-col gap-4'>
        <div className='flex items-center gap-2'>
          <Typography.Text>{t('时间范围')}:</Typography.Text>
          <DatePicker
            type='dateTimeRange'
            value={dateRange}
            onChange={(val) => {
              if (val && val.length === 2) {
                setDateRange(val);
              }
            }}
            style={{ width: 380 }}
          />
        </div>

        <Spin spinning={loading}>
          {hasData ? (
            <Tabs
              type='line'
              activeKey={activeTab}
              onChange={setActiveTab}
            >
              <TabPane tab={t('额度消耗')} itemKey='quota'>
                <div style={{ height: 360 }}>
                  <VChart spec={quotaSpec} option={CHART_CONFIG} />
                </div>
              </TabPane>
              <TabPane tab={t('Token 消耗')} itemKey='token'>
                <div style={{ height: 360 }}>
                  <VChart spec={tokenSpec} option={CHART_CONFIG} />
                </div>
              </TabPane>
              <TabPane tab={t('请求次数')} itemKey='count'>
                <div style={{ height: 360 }}>
                  <VChart spec={countSpec} option={CHART_CONFIG} />
                </div>
              </TabPane>
            </Tabs>
          ) : (
            !loading && (
              <Empty
                title={t('暂无数据')}
                description={t('该令牌在所选时间范围内没有使用记录')}
              />
            )
          )}
        </Spin>
      </div>
    </SideSheet>
  );
};

export default TokenStatsModal;
