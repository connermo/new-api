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
import { Card, Typography, Space, Divider } from '@douyinfe/semi-ui';
import { Activity, Zap, ArrowUpCircle, ArrowDownCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * TokenUsageDisplay component for displaying token usage statistics and throughput
 * @param {Object} props - Component props
 * @param {Object} props.usage - Token usage data from API response
 * @param {number} props.usage.prompt_tokens - Number of tokens in the prompt
 * @param {number} props.usage.completion_tokens - Number of tokens in the completion
 * @param {number} props.usage.total_tokens - Total number of tokens
 * @param {string} props.tokensPerSecond - Throughput rate in tokens per second
 * @returns {JSX.Element} Rendered token usage display component
 */
const TokenUsageDisplay = ({ usage, tokensPerSecond }) => {
  const { t } = useTranslation();

  if (!usage) {
    return null;
  }

  return (
    <Card
      className='w-full'
      bordered={true}
      bodyStyle={{
        padding: '12px 16px',
      }}
    >
      <Space vertical align='start' spacing='tight' className='w-full'>
        <div className='flex items-center gap-2'>
          <Activity size={16} className='text-blue-500' />
          <Typography.Text strong className='text-sm'>
            {t('Token使用统计')}
          </Typography.Text>
        </div>

        <div className='flex items-center justify-between w-full gap-4'>
          <div className='flex items-center gap-2'>
            <ArrowUpCircle size={14} className='text-green-600' />
            <Typography.Text className='text-xs text-gray-600 dark:text-gray-400'>
              {t('输入')}:
            </Typography.Text>
            <Typography.Text strong className='text-sm'>
              {usage.prompt_tokens || 0}
            </Typography.Text>
          </div>

          <Divider layout='vertical' margin='0' />

          <div className='flex items-center gap-2'>
            <ArrowDownCircle size={14} className='text-purple-600' />
            <Typography.Text className='text-xs text-gray-600 dark:text-gray-400'>
              {t('输出')}:
            </Typography.Text>
            <Typography.Text strong className='text-sm'>
              {usage.completion_tokens || 0}
            </Typography.Text>
          </div>

          <Divider layout='vertical' margin='0' />

          <div className='flex items-center gap-2'>
            <Typography.Text className='text-xs text-gray-600 dark:text-gray-400'>
              {t('总计')}:
            </Typography.Text>
            <Typography.Text strong className='text-sm text-blue-600'>
              {usage.total_tokens || 0}
            </Typography.Text>
          </div>

          {tokensPerSecond && (
            <>
              <Divider layout='vertical' margin='0' />
              <div className='flex items-center gap-2'>
                <Zap size={14} className='text-orange-500' />
                <Typography.Text className='text-xs text-gray-600 dark:text-gray-400'>
                  {t('吞吐')}:
                </Typography.Text>
                <Typography.Text strong className='text-sm text-orange-600'>
                  {tokensPerSecond} {t('tokens/s')}
                </Typography.Text>
              </div>
            </>
          )}
        </div>
      </Space>
    </Card>
  );
};

export default TokenUsageDisplay;
