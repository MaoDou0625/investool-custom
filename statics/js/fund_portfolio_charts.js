(function () {
  "use strict";

  var chartInstances = [];
  var palette = [
    "#2563eb",
    "#059669",
    "#d97706",
    "#dc2626",
    "#7c3aed",
    "#0891b2",
    "#4b5563",
    "#be185d",
    "#65a30d",
    "#9333ea",
  ];

  function onReady(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
      return;
    }
    fn();
  }

  function formatPercent(value, digits) {
    var num = Number(value || 0);
    return num.toFixed(digits == null ? 1 : digits) + "%";
  }

  function formatAmount(value) {
    return Number(value || 0).toFixed(2);
  }

  function trimLabel(value, maxLength) {
    var text = String(value || "");
    if (text.length <= maxLength) {
      return text;
    }
    return text.slice(0, maxLength - 1) + "...";
  }

  function initChart(id) {
    var el = document.getElementById(id);
    if (!el || !window.echarts) {
      return null;
    }
    var chart = echarts.init(el);
    chartInstances.push(chart);
    return chart;
  }

  function showUnavailable(id) {
    var el = document.getElementById(id);
    if (el) {
      el.innerHTML = '<p class="grey-text center-align">图表库加载失败，表格数据仍可查看。</p>';
    }
  }

  function renderThemeExposure(data) {
    var chart = initChart("fund-theme-exposure-chart");
    if (!chart) {
      return;
    }
    var rows = (data.themes || []).slice(0, 12).reverse();
    chart.setOption({
      color: palette,
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        formatter: function (params) {
          var row = rows[params[0].dataIndex];
          return [
            "<strong>" + row.name + "</strong>",
            "组合占比：" + formatPercent(row.weight, 1),
            "估算金额：" + formatAmount(row.amount),
            "主要来源：" + row.sources,
          ].join("<br>");
        },
      },
      grid: { top: 12, right: 48, bottom: 24, left: 112 },
      xAxis: {
        type: "value",
        axisLabel: { formatter: "{value}%" },
        splitLine: { lineStyle: { color: "#eceff1" } },
      },
      yAxis: {
        type: "category",
        data: rows.map(function (row) { return row.name; }),
        axisLabel: {
          formatter: function (value) { return trimLabel(value, 9); },
        },
      },
      series: [
        {
          name: "组合占比",
          type: "bar",
          data: rows.map(function (row, index) {
            return {
              value: Number(row.weight || 0).toFixed(2),
              itemStyle: { color: palette[index % palette.length] },
            };
          }),
          barMaxWidth: 24,
          label: {
            show: true,
            position: "right",
            formatter: function (params) {
              return formatPercent(params.value, 1);
            },
          },
        },
      ],
    });
  }

  function renderETFLookThrough(data) {
    var chart = initChart("fund-etf-lookthrough-chart");
    if (!chart) {
      return;
    }
    var links = (data.etfLinks || []).map(function (link) {
      return {
        source: trimLabel(link.sourceName, 16),
        target: trimLabel(link.targetName, 16),
        value: Number(link.weight || 0),
        raw: link,
      };
    });
    if (links.length === 0) {
      chart.setOption({
        title: {
          text: "暂无可穿透ETF",
          left: "center",
          top: "middle",
          textStyle: { color: "#9e9e9e", fontSize: 14, fontWeight: "normal" },
        },
      });
      return;
    }

    var nodeMap = {};
    links.forEach(function (link) {
      nodeMap[link.source] = true;
      nodeMap[link.target] = true;
    });
    var nodes = Object.keys(nodeMap).map(function (name) {
      return { name: name };
    });

    chart.setOption({
      color: palette,
      tooltip: {
        trigger: "item",
        triggerOn: "mousemove",
        formatter: function (params) {
          if (params.dataType !== "edge") {
            return params.name;
          }
          var raw = params.data.raw || {};
          return [
            "<strong>" + raw.sourceName + "</strong>",
            "目标ETF：" + raw.targetName,
            "组合占比：" + formatPercent(raw.weight, 1),
            "当前金额：" + formatAmount(raw.amount),
            "状态：" + raw.status,
          ].join("<br>");
        },
      },
      series: [
        {
          type: "sankey",
          data: nodes,
          links: links,
          left: 4,
          right: 112,
          top: 16,
          bottom: 16,
          nodeWidth: 12,
          nodeGap: 12,
          draggable: false,
          label: {
            fontSize: 10,
            color: "#374151",
            overflow: "truncate",
            width: 104,
          },
          lineStyle: { color: "gradient", opacity: 0.35, curveness: 0.5 },
          emphasis: { focus: "adjacency" },
        },
      ],
    });
  }

  function renderStockExposure(data) {
    var chart = initChart("fund-stock-exposure-chart");
    if (!chart) {
      return;
    }
    var rows = (data.stocks || []).slice(0, 10).reverse();
    chart.setOption({
      color: palette,
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        formatter: function (params) {
          var row = rows[params[0].dataIndex];
          return [
            "<strong>" + row.name + " " + row.code + "</strong>",
            "主题：" + row.theme,
            "行业：" + row.industry,
            "组合占比：" + formatPercent(row.weight, 2),
            "估算金额：" + formatAmount(row.amount),
            "来源：" + row.source,
          ].join("<br>");
        },
      },
      grid: { top: 12, right: 52, bottom: 24, left: 108 },
      xAxis: {
        type: "value",
        axisLabel: { formatter: "{value}%" },
        splitLine: { lineStyle: { color: "#eceff1" } },
      },
      yAxis: {
        type: "category",
        data: rows.map(function (row) { return row.name; }),
        axisLabel: {
          formatter: function (value) { return trimLabel(value, 8); },
        },
      },
      series: [
        {
          name: "组合占比",
          type: "bar",
          data: rows.map(function (row, index) {
            return {
              value: Number(row.weight || 0).toFixed(2),
              itemStyle: { color: palette[index % palette.length] },
            };
          }),
          barMaxWidth: 22,
          label: {
            show: true,
            position: "right",
            formatter: function (params) {
              return formatPercent(params.value, 2);
            },
          },
        },
      ],
    });
  }

  onReady(function () {
    var data = window.fundPortfolioChartData;
    if (!data) {
      return;
    }
    if (!window.echarts) {
      showUnavailable("fund-theme-exposure-chart");
      showUnavailable("fund-etf-lookthrough-chart");
      showUnavailable("fund-stock-exposure-chart");
      return;
    }

    renderThemeExposure(data);
    renderETFLookThrough(data);
    renderStockExposure(data);
    window.addEventListener("resize", function () {
      chartInstances.forEach(function (chart) {
        chart.resize();
      });
    });
  });
})();
