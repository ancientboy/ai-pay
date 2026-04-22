"use client";

import ReactECharts from "echarts-for-react";

export function MetricsTrendChart({
  todaySpend,
  successRate,
  title = "Trend",
  spendLabel = "Today Spend",
  successLabel = "Success Rate",
}: {
  todaySpend: number;
  successRate: number;
  title?: string;
  spendLabel?: string;
  successLabel?: string;
}) {
  const option = {
    backgroundColor: "transparent",
    tooltip: { trigger: "axis" },
    legend: {
      textStyle: { color: "#9FB0CC" },
      top: 0,
    },
    grid: { left: 32, right: 20, top: 32, bottom: 24 },
    xAxis: {
      type: "category",
      data: ["00", "04", "08", "12", "16", "20", "24"],
      axisLabel: { color: "#9FB0CC" },
      axisLine: { lineStyle: { color: "#334155" } },
    },
    yAxis: [
      {
        type: "value",
        name: "Spend",
        axisLabel: { color: "#9FB0CC" },
        axisLine: { lineStyle: { color: "#334155" } },
        splitLine: { lineStyle: { color: "#1F2937" } },
      },
      {
        type: "value",
        name: "Success%",
        min: 0,
        max: 100,
        axisLabel: { color: "#9FB0CC" },
        axisLine: { lineStyle: { color: "#334155" } },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: spendLabel,
        type: "line",
        smooth: true,
        data: [0, todaySpend * 0.1, todaySpend * 0.35, todaySpend * 0.5, todaySpend * 0.7, todaySpend * 0.9, todaySpend],
        lineStyle: { color: "#2F6BFF", width: 2 },
        areaStyle: { color: "rgba(47,107,255,0.18)" },
      },
      {
        name: successLabel,
        type: "line",
        yAxisIndex: 1,
        smooth: true,
        data: [
          Math.max(successRate - 5, 80),
          Math.max(successRate - 3, 82),
          Math.max(successRate - 2, 84),
          Math.max(successRate - 1, 85),
          Math.max(successRate - 1, 86),
          successRate,
          successRate,
        ],
        lineStyle: { color: "#12B76A", width: 2 },
      },
    ],
  };

  return (
    <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
      <h3 className="mb-2 text-sm font-medium text-slate-200">{title}</h3>
      <ReactECharts option={option} style={{ height: 260 }} notMerge />
    </div>
  );
}
