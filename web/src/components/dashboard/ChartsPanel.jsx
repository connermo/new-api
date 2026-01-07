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

import React from 'react';
import { Card, Tabs, TabPane, Table } from '@douyinfe/semi-ui';
import { PieChart } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';
import { renderNumber, renderQuota } from '../../helpers';

const ChartsPanel = ({
  activeChartTab,
  setActiveChartTab,
  spec_line,
  spec_model_line,
  spec_pie,
  spec_rank_bar,
  modelTableData,
  CARD_PROPS,
  CHART_CONFIG,
  FLEX_CENTER_GAP2,
  hasApiInfoPanel,
  t,
}) => {
  const tableColumns = [
    {
      title: t('模型名称'),
      dataIndex: 'model_name',
      key: 'model_name',
      fixed: 'left',
    },
    {
      title: t('调用次数'),
      dataIndex: 'call_count',
      key: 'call_count',
      sorter: (a, b) => a.call_count - b.call_count,
      render: (text) => renderNumber(text),
    },
    {
      title: t('总Token消耗'),
      dataIndex: 'total_tokens',
      key: 'total_tokens',
      sorter: (a, b) => a.total_tokens - b.total_tokens,
      render: (text) => renderNumber(text),
    },
    {
      title: t('模型描述'),
      dataIndex: 'total_quota',
      key: 'total_quota',
      sorter: (a, b) => a.total_quota - b.total_quota,
      render: (text) => renderQuota(text, 4),
    },
  ];

  return (
    <Card
      {...CARD_PROPS}
      className={`!rounded-2xl ${hasApiInfoPanel ? 'lg:col-span-3' : ''}`}
      title={
        <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between w-full gap-3'>
          <div className={FLEX_CENTER_GAP2}>
            <PieChart size={16} />
            {t('模型数据分析')}
          </div>
          <Tabs
            type='slash'
            activeKey={activeChartTab}
            onChange={setActiveChartTab}
          >
            <TabPane tab={<span>{t('消耗分布')}</span>} itemKey='1' />
            <TabPane tab={<span>{t('消耗趋势')}</span>} itemKey='2' />
            <TabPane tab={<span>{t('调用次数分布')}</span>} itemKey='3' />
            <TabPane tab={<span>{t('调用次数排行')}</span>} itemKey='4' />
            <TabPane tab={<span>{t('模型统计列表')}</span>} itemKey='5' />
          </Tabs>
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      <div className='h-96 p-2'>
        {activeChartTab === '1' && (
          <VChart spec={spec_line} option={CHART_CONFIG} />
        )}
        {activeChartTab === '2' && (
          <VChart spec={spec_model_line} option={CHART_CONFIG} />
        )}
        {activeChartTab === '3' && (
          <VChart spec={spec_pie} option={CHART_CONFIG} />
        )}
        {activeChartTab === '4' && (
          <VChart spec={spec_rank_bar} option={CHART_CONFIG} />
        )}
        {activeChartTab === '5' && (
          <div className='h-full overflow-auto'>
            <Table
              columns={tableColumns}
              dataSource={modelTableData}
              pagination={{
                pageSize: 10,
                showSizeChanger: true,
                pageSizeOpts: [10, 20, 50, 100],
              }}
              size='small'
            />
          </div>
        )}
      </div>
    </Card>
  );
};

export default ChartsPanel;
