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
import { Progress, Divider, Empty } from '@douyinfe/semi-ui';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import {
  timestamp2string,
  timestamp2string1,
  isDataCrossYear,
  copy,
  showSuccess,
} from './utils';
import {
  STORAGE_KEYS,
  DEFAULT_TIME_INTERVALS,
  DEFAULTS,
  ILLUSTRATION_SIZE,
} from '../constants/dashboard.constants';
import { renderQuota, renderNumber } from './render';

// ========== 时间相关工具函数 ==========
export const getDefaultTime = () => {
  return localStorage.getItem(STORAGE_KEYS.DATA_EXPORT_DEFAULT_TIME) || 'hour';
};

export const getTimeInterval = (timeType, isSeconds = false) => {
  const intervals =
    DEFAULT_TIME_INTERVALS[timeType] || DEFAULT_TIME_INTERVALS.hour;
  return isSeconds ? intervals.seconds : intervals.minutes;
};

export const getInitialTimestamp = () => {
  const defaultTime = getDefaultTime();
  const now = new Date().getTime() / 1000;

  switch (defaultTime) {
    case 'hour':
      return timestamp2string(now - 86400);
    case 'week':
      return timestamp2string(now - 86400 * 30);
    default:
      return timestamp2string(now - 86400 * 7);
  }
};

// ========== 数据处理工具函数 ==========
export const updateMapValue = (map, key, value) => {
  if (!map.has(key)) {
    map.set(key, 0);
  }
  map.set(key, map.get(key) + value);
};

export const initializeMaps = (key, ...maps) => {
  maps.forEach((map) => {
    if (!map.has(key)) {
      map.set(key, 0);
    }
  });
};

// ========== 图表相关工具函数 ==========
export const updateChartSpec = (
  setterFunc,
  newData,
  subtitle,
  newColors,
  dataId,
) => {
  setterFunc((prev) => ({
    ...prev,
    data: [{ id: dataId, values: newData }],
    title: {
      ...prev.title,
      subtext: subtitle,
    },
    color: {
      specified: newColors,
    },
  }));
};

export const getTrendSpec = (data, color) => ({
  type: 'line',
  data: [{ id: 'trend', values: data.map((val, idx) => ({ x: idx, y: val })) }],
  xField: 'x',
  yField: 'y',
  height: 40,
  width: 100,
  axes: [
    {
      orient: 'bottom',
      visible: false,
    },
    {
      orient: 'left',
      visible: false,
    },
  ],
  padding: 0,
  autoFit: false,
  legends: { visible: false },
  tooltip: { visible: false },
  crosshair: { visible: false },
  line: {
    style: {
      stroke: color,
      lineWidth: 2,
    },
  },
  point: {
    visible: false,
  },
  background: {
    fill: 'transparent',
  },
});

// ========== UI 工具函数 ==========
export const createSectionTitle = (Icon, text) => (
  <div className='flex items-center gap-2'>
    <Icon size={16} />
    {text}
  </div>
);

export const createFormField = (Component, props, FORM_FIELD_PROPS) => (
  <Component {...FORM_FIELD_PROPS} {...props} />
);

// ========== 操作处理函数 ==========
export const handleCopyUrl = async (url, t) => {
  if (await copy(url)) {
    showSuccess(t('复制成功'));
  }
};

export const handleSpeedTest = (apiUrl) => {
  const encodedUrl = encodeURIComponent(apiUrl);
  const speedTestUrl = `https://www.tcptest.cn/http/${encodedUrl}`;
  window.open(speedTestUrl, '_blank', 'noopener,noreferrer');
};

// ========== 状态映射函数 ==========
export const getUptimeStatusColor = (status, uptimeStatusMap) =>
  uptimeStatusMap[status]?.color || '#8b9aa7';

export const getUptimeStatusText = (status, uptimeStatusMap, t) =>
  uptimeStatusMap[status]?.text || t('未知');

// ========== 监控列表渲染函数 ==========
export const renderMonitorList = (
  monitors,
  getUptimeStatusColor,
  getUptimeStatusText,
  t,
) => {
  if (!monitors || monitors.length === 0) {
    return (
      <div className='flex justify-center items-center py-4'>
        <Empty
          image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
          darkModeImage={
            <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
          }
          title={t('暂无监控数据')}
        />
      </div>
    );
  }

  const grouped = {};
  monitors.forEach((m) => {
    const g = m.group || '';
    if (!grouped[g]) grouped[g] = [];
    grouped[g].push(m);
  });

  const renderItem = (monitor, idx) => (
    <div key={idx} className='p-2 hover:bg-white rounded-lg transition-colors'>
      <div className='flex items-center justify-between mb-1'>
        <div className='flex items-center gap-2'>
          <div
            className='w-2 h-2 rounded-full flex-shrink-0'
            style={{ backgroundColor: getUptimeStatusColor(monitor.status) }}
          />
          <span className='text-sm font-medium text-gray-900'>
            {monitor.name}
          </span>
        </div>
        <span className='text-xs text-gray-500'>
          {((monitor.uptime || 0) * 100).toFixed(2)}%
        </span>
      </div>
      <div className='flex items-center gap-2'>
        <span className='text-xs text-gray-500'>
          {getUptimeStatusText(monitor.status)}
        </span>
        <div className='flex-1'>
          <Progress
            percent={(monitor.uptime || 0) * 100}
            showInfo={false}
            aria-label={`${monitor.name} uptime`}
            stroke={getUptimeStatusColor(monitor.status)}
          />
        </div>
      </div>
    </div>
  );

  return Object.entries(grouped).map(([gname, list]) => (
    <div key={gname || 'default'} className='mb-2'>
      {gname && (
        <>
          <div className='text-md font-semibold text-gray-500 px-2 py-1'>
            {gname}
          </div>
          <Divider />
        </>
      )}
      {list.map(renderItem)}
    </div>
  ));
};

// ========== 数据处理函数 ==========
export const processRawData = (
  data,
  dataExportDefaultTime,
  initializeMaps,
  updateMapValue,
) => {
  const result = {
    totalQuota: 0,
    totalTimes: 0,
    totalTokens: 0,
    uniqueModels: new Set(),
    timePoints: [],
    timeQuotaMap: new Map(),
    timeTokensMap: new Map(),
    timeCountMap: new Map(),
  };

  // 检查数据是否跨年
  const showYear = isDataCrossYear(data.map((item) => item.created_at));

  data.forEach((item) => {
    result.uniqueModels.add(item.model_name);
    result.totalTokens += item.token_used;
    result.totalQuota += item.quota;
    result.totalTimes += item.count;

    const timeKey = timestamp2string1(
      item.created_at,
      dataExportDefaultTime,
      showYear,
    );
    if (!result.timePoints.includes(timeKey)) {
      result.timePoints.push(timeKey);
    }

    initializeMaps(
      timeKey,
      result.timeQuotaMap,
      result.timeTokensMap,
      result.timeCountMap,
    );
    updateMapValue(result.timeQuotaMap, timeKey, item.quota);
    updateMapValue(result.timeTokensMap, timeKey, item.token_used);
    updateMapValue(result.timeCountMap, timeKey, item.count);
  });

  result.timePoints.sort();
  return result;
};

export const calculateTrendData = (
  timePoints,
  timeQuotaMap,
  timeTokensMap,
  timeCountMap,
  dataExportDefaultTime,
) => {
  const quotaTrend = timePoints.map((time) => timeQuotaMap.get(time) || 0);
  const tokensTrend = timePoints.map((time) => timeTokensMap.get(time) || 0);
  const countTrend = timePoints.map((time) => timeCountMap.get(time) || 0);

  const rpmTrend = [];
  const tpmTrend = [];

  if (timePoints.length >= 2) {
    const interval = getTimeInterval(dataExportDefaultTime);

    for (let i = 0; i < timePoints.length; i++) {
      rpmTrend.push(timeCountMap.get(timePoints[i]) / interval);
      tpmTrend.push(timeTokensMap.get(timePoints[i]) / interval);
    }
  }

  return {
    balance: [],
    usedQuota: [],
    requestCount: [],
    times: countTrend,
    consumeQuota: quotaTrend,
    tokens: tokensTrend,
    rpm: rpmTrend,
    tpm: tpmTrend,
  };
};

export const aggregateDataByTimeAndModel = (data, dataExportDefaultTime) => {
  const aggregatedData = new Map();

  // 检查数据是否跨年
  const showYear = isDataCrossYear(data.map((item) => item.created_at));

  data.forEach((item) => {
    const timeKey = timestamp2string1(
      item.created_at,
      dataExportDefaultTime,
      showYear,
    );
    const modelKey = item.model_name;
    const key = `${timeKey}-${modelKey}`;

    if (!aggregatedData.has(key)) {
      aggregatedData.set(key, {
        time: timeKey,
        model: modelKey,
        quota: 0,
        count: 0,
      });
    }

    const existing = aggregatedData.get(key);
    existing.quota += item.quota;
    existing.count += item.count;
  });

  return aggregatedData;
};

export const generateChartTimePoints = (
  aggregatedData,
  data,
  dataExportDefaultTime,
) => {
  let chartTimePoints = Array.from(
    new Set([...aggregatedData.values()].map((d) => d.time)),
  );

  if (chartTimePoints.length < DEFAULTS.MAX_TREND_POINTS) {
    const lastTime = Math.max(...data.map((item) => item.created_at));
    const interval = getTimeInterval(dataExportDefaultTime, true);

    // 生成时间点数组，用于检查是否跨年
    const generatedTimestamps = Array.from(
      { length: DEFAULTS.MAX_TREND_POINTS },
      (_, i) => lastTime - (6 - i) * interval,
    );
    const showYear = isDataCrossYear(generatedTimestamps);

    chartTimePoints = generatedTimestamps.map((ts) =>
      timestamp2string1(ts, dataExportDefaultTime, showYear),
    );
  }

  return chartTimePoints;
};

// ========== 用户维度数据处理 ==========
export const processUserData = (data, dataExportDefaultTime, limit = 10) => {
  const userQuotaTotal = new Map();
  data.forEach((item) => {
    const prev = userQuotaTotal.get(item.username) || 0;
    userQuotaTotal.set(item.username, prev + item.quota);
  });

  const sorted = Array.from(userQuotaTotal.entries()).sort(
    (a, b) => b[1] - a[1],
  );
  const topUsers = sorted.slice(0, limit).map(([u]) => u);
  const topUserSet = new Set(topUsers);

  const rankingData = sorted.slice(0, limit).map(([username, quota]) => ({
    User: username,
    Quota: quota,
  }));

  const showYear = isDataCrossYear(data.map((item) => item.created_at));

  const timeUserMap = new Map();
  const allTimePoints = new Set();

  data.forEach((item) => {
    const timeKey = timestamp2string1(
      item.created_at,
      dataExportDefaultTime,
      showYear,
    );
    allTimePoints.add(timeKey);
    const user = topUserSet.has(item.username) ? item.username : null;
    if (!user) return;
    const key = `${timeKey}-${user}`;
    const prev = timeUserMap.get(key) || { quota: 0 };
    timeUserMap.set(key, { quota: prev.quota + item.quota });
  });

  const sortedTimePoints = Array.from(allTimePoints).sort();
  const trendData = [];
  sortedTimePoints.forEach((time) => {
    topUsers.forEach((user) => {
      const key = `${time}-${user}`;
      const val = timeUserMap.get(key);
      trendData.push({
        Time: time,
        User: user,
        Quota: val?.quota || 0,
      });
    });
  });

  return { rankingData, trendData, topUsers };
};

export const processUserModelData = (data, t, limit = 10) => {
  const userQuotaTotal = new Map();
  data.forEach((item) => {
    const prev = userQuotaTotal.get(item.username) || 0;
    userQuotaTotal.set(item.username, prev + (Number(item.quota) || 0));
  });

  const sortedUsers = Array.from(userQuotaTotal.entries()).sort(
    (a, b) => b[1] - a[1],
  );
  const topUsers = sortedUsers.slice(0, limit).map(([u]) => u);
  const topUserSet = new Set(topUsers);

  const allModels = new Set();
  data.forEach((item) => {
    if (topUserSet.has(item.username)) {
      allModels.add(item.model_name || 'Unknown');
    }
  });
  const modelList = Array.from(allModels).sort();

  const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit')) || 1;
  const userModelData = new Map();
  data.forEach((item) => {
    if (!topUserSet.has(item.username)) return;

    const model = item.model_name || 'Unknown';
    const key = `${item.username}-${model}`;
    if (!userModelData.has(key)) {
      userModelData.set(key, {
        User: item.username,
        Model: model,
        rawQuota: 0,
        Quota: 0,
        Count: 0,
      });
    }
    const existing = userModelData.get(key);
    existing.rawQuota += Number(item.quota) || 0;
    existing.Quota = existing.rawQuota / quotaPerUnit;
    existing.Count += Number(item.count) || 0;
  });

  const quotaData = [];
  const countData = [];

  topUsers.forEach((user) => {
    modelList.forEach((model) => {
      const key = `${user}-${model}`;
      const d = userModelData.get(key);
      quotaData.push({
        User: user,
        Model: model,
        rawQuota: d?.rawQuota || 0,
        Quota: d?.Quota || 0,
        Count: d?.Count || 0,
      });
      countData.push({
        User: user,
        Model: model,
        rawQuota: d?.rawQuota || 0,
        Quota: d?.Quota || 0,
        Count: d?.Count || 0,
      });
    });
  });

  const goldenAngle = 137.508;

  const hslToHex = (h, s, l) => {
    s /= 100;
    l /= 100;
    const a = s * Math.min(l, 1 - l);
    const f = (n) => {
      const k = (n + h / 30) % 12;
      const color = l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1);
      return Math.round(255 * color)
        .toString(16)
        .padStart(2, '0');
    };
    return `#${f(0)}${f(8)}${f(4)}`;
  };

  // 生成最大化颜色差异的色板
  // 策略：均匀分布色相 + 交替饱和度 + 交替亮度
  const generateDistinctColors = (count) => {
    const colors = {};
    const satOptions = [75, 60, 85, 50, 70]; // 5种饱和度
    const litOptions = [50, 40, 60, 45, 55]; // 5种亮度

    for (let i = 0; i < count; i++) {
      // 使用黄金角度分布色相，确保相邻模型颜色差异最大
      const hue = Math.round((i * 137.508) % 360);
      // 交替饱和度和亮度，增加颜色区分度
      const sat = satOptions[i % satOptions.length];
      const lit = litOptions[Math.floor(i / 5) % litOptions.length];
      const model = modelList[i];
      colors[model] = hslToHex(hue, sat, lit);
    }
    return colors;
  };

  const colorMap = generateDistinctColors(modelList.length);

  const colorConfig = {
    specified: colorMap,
  };

  const totalQuota = quotaData.reduce((sum, item) => sum + item.rawQuota, 0);
  const totalCount = countData.reduce((sum, item) => sum + item.Count, 0);

  const userModelQuotaSpec = {
    data: [{ id: 'userModelQuotaData', values: quotaData }],
    title: {
      visible: true,
      text: t('用户模型消耗排行'),
      subtext: `${t('总计')}：${renderQuota(totalQuota, 2)}`,
    },
    color: colorConfig,
  };

  const userModelCountSpec = {
    data: [{ id: 'userModelCountData', values: countData }],
    title: {
      visible: true,
      text: t('用户模型调用次数排行'),
      subtext: `${t('总计')}：${renderNumber(totalCount)}`,
    },
    color: colorConfig,
  };

  return { userModelQuotaSpec, userModelCountSpec };
};
