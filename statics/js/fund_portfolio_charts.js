(function () {
  "use strict";

  var chartInstances = {};
  var chartIds = [
    "fund-theme-exposure-chart",
    "fund-stock-exposure-chart",
    "fund-risk-return-chart",
    "fund-portfolio-history-chart",
    "fund-nav-trend-chart",
    "fund-correlation-heatmap-chart",
    "fund-comparison-radar-chart",
  ];
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

  function formatMoney(value) {
    return formatAmount(value) + " 元";
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
    if (chartInstances[id]) {
      return chartInstances[id];
    }
    var chart = echarts.init(el);
    chartInstances[id] = chart;
    return chart;
  }

  function renderEmpty(id, message) {
    var chart = initChart(id);
    if (!chart) {
      return;
    }
    chart.clear();
    chart.setOption({
      title: {
        text: message,
        left: "center",
        top: "middle",
        textStyle: { color: "#78909c", fontSize: 14, fontWeight: 400 },
      },
    });
  }

  function showUnavailable(id) {
    var el = document.getElementById(id);
    if (el) {
      el.innerHTML = '<p class="grey-text center-align">图表库加载失败，请刷新后重试。</p>';
    }
  }

  function resizeCharts() {
    Object.keys(chartInstances).forEach(function (id) {
      chartInstances[id].resize();
    });
  }

  function sumBy(rows, fieldName) {
    var total = 0;
    rows.forEach(function (row) {
      total += Number(row[fieldName] || 0);
    });
    return total;
  }

  function uniqueThemeSources(rows) {
    var totals = {};
    rows.forEach(function (row) {
      if (!row.source) {
        return;
      }
      totals[row.source] = (totals[row.source] || 0) + Number(row.weight || 0);
    });
    return Object.keys(totals).sort(function (a, b) {
      return totals[b] - totals[a];
    });
  }

  function renderThemeExposure(data) {
    var themes = (data.themes || []).slice(0, 12);
    if (themes.length === 0) {
      renderEmpty("fund-theme-exposure-chart", "暂无主题暴露数据");
      return;
    }

    var chart = initChart("fund-theme-exposure-chart");
    if (!chart) {
      return;
    }
    var rows = themes.reverse();
    var themeByName = {};
    rows.forEach(function (row) {
      themeByName[row.name] = row;
    });

    var sourceRows = (data.themeSources || []).filter(function (row) {
      return themeByName[row.theme];
    });
    var sources = uniqueThemeSources(sourceRows);
    var series = [];
    if (sources.length > 0) {
      series = sources.map(function (source) {
        return {
          name: source,
          type: "bar",
          stack: "theme-source",
          barMaxWidth: 24,
          emphasis: { focus: "series" },
          data: rows.map(function (theme) {
            var match = sourceRows.find(function (row) {
              return row.theme === theme.name && row.source === source;
            });
            return match ? Number(match.weight || 0).toFixed(2) : 0;
          }),
        };
      });
    } else {
      series = [{
        name: "组合占比",
        type: "bar",
        barMaxWidth: 24,
        data: rows.map(function (row, index) {
          return {
            value: Number(row.weight || 0).toFixed(2),
            itemStyle: { color: palette[index % palette.length] },
          };
        }),
      }];
    }

    chart.setOption({
      color: palette,
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        formatter: function (params) {
          var row = rows[params[0].dataIndex];
          var lines = [
            "<strong>" + row.name + "</strong>",
            "组合占比：" + formatPercent(row.weight, 1),
            "估算金额：" + formatMoney(row.amount),
          ];
          var sourceLines = params.filter(function (item) {
            return Number(item.value || 0) > 0;
          }).map(function (item) {
            return item.seriesName + "：" + formatPercent(item.value, 1);
          });
          if (sourceLines.length > 0) {
            lines.push("来源拆分：");
            lines = lines.concat(sourceLines);
          } else if (row.sources) {
            lines.push("主要来源：" + row.sources);
          }
          return lines.join("<br>");
        },
      },
      legend: {
        type: "scroll",
        top: 0,
        left: 8,
        right: 8,
        itemWidth: 10,
        itemHeight: 10,
      },
      grid: { top: sources.length > 0 ? 54 : 16, right: 48, bottom: 24, left: 112, containLabel: true },
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
      series: series,
    });
  }

  function renderStockExposure(data) {
    var rows = (data.stocks || []).slice(0, 10).reverse();
    if (rows.length === 0) {
      renderEmpty("fund-stock-exposure-chart", "暂无穿透后重仓股数据");
      return;
    }

    var chart = initChart("fund-stock-exposure-chart");
    if (!chart) {
      return;
    }
    var concentration = data.stockConcentration || {};
    var subtitle = "Top10 集中度 " + formatPercent(concentration.top10Weight, 2);
    if (concentration.largestName) {
      subtitle += "；第一大 " + concentration.largestName + " " + formatPercent(concentration.largestWeight, 2);
    }

    chart.setOption({
      color: palette,
      title: {
        text: "穿透后重仓股",
        subtext: subtitle,
        left: 8,
        top: 0,
        textStyle: { fontSize: 14, fontWeight: 500 },
        subtextStyle: { color: "#607d8b" },
      },
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
            "估算金额：" + formatMoney(row.amount),
            "来源：" + row.source,
          ].join("<br>");
        },
      },
      grid: { top: 64, right: 52, bottom: 24, left: 108, containLabel: true },
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

  function renderRiskReturn(data) {
    var rows = (data.riskReturns || []);
    if (rows.length === 0) {
      renderEmpty("fund-risk-return-chart", "暂无可用于风险收益散点的数据");
      return;
    }

    var chart = initChart("fund-risk-return-chart");
    if (!chart) {
      return;
    }
    var maxAmount = Math.max(sumBy(rows, "currentAmount"), 1);
    var scatterData = rows.map(function (row) {
      return [row.risk, row.expectedReturn, row.currentAmount, row.name, row.code, row.status, row.action, row.score, row.currentWeight, row.returnLabel];
    });

    chart.setOption({
      color: ["#2563eb"],
      tooltip: {
        trigger: "item",
        formatter: function (params) {
          var value = params.value;
          return [
            "<strong>" + value[3] + " " + value[4] + "</strong>",
            "状态：" + value[5] + "；操作：" + value[6],
            "评分：" + value[7],
            "风险：" + formatPercent(value[0], 2),
            String(value[9] || "收益") + "：" + formatPercent(value[1], 2),
            "当前总值：" + formatMoney(value[2]),
            "当前仓位：" + formatPercent(value[8], 1),
          ].join("<br>");
        },
      },
      grid: { top: 28, right: 28, bottom: 46, left: 58, containLabel: true },
      xAxis: {
        type: "value",
        name: "风险/回撤",
        nameLocation: "middle",
        nameGap: 28,
        axisLabel: { formatter: "{value}%" },
        splitLine: { lineStyle: { color: "#eceff1" } },
        scale: true,
      },
      yAxis: {
        type: "value",
        name: "预期或近1年收益",
        axisLabel: { formatter: "{value}%" },
        splitLine: { lineStyle: { color: "#eceff1" } },
        scale: true,
      },
      series: [
        {
          name: "基金",
          type: "scatter",
          data: scatterData,
          symbolSize: function (value) {
            var amount = Number(value[2] || 0);
            return Math.max(10, Math.min(34, 10 + Math.sqrt(amount / maxAmount) * 24));
          },
          label: {
            show: true,
            position: "right",
            formatter: function (params) {
              return trimLabel(params.value[4], 8);
            },
          },
          itemStyle: { opacity: 0.82 },
        },
      ],
    });
  }

  function renderHistory(data) {
    var rows = (data.history || []);
    if (rows.length === 0) {
      renderEmpty("fund-portfolio-history-chart", "暂无组合历史快照");
      return;
    }

    var chart = initChart("fund-portfolio-history-chart");
    if (!chart) {
      return;
    }
    var subtitle = rows.length < 2 ? "已记录今天快照，多日后会形成曲线" : "";
    chart.setOption({
      color: ["#2563eb", "#dc2626"],
      title: {
        text: "组合总值与收益率",
        subtext: subtitle,
        left: 8,
        top: 0,
        textStyle: { fontSize: 14, fontWeight: 500 },
        subtextStyle: { color: "#607d8b" },
      },
      tooltip: {
        trigger: "axis",
        formatter: function (params) {
          var idx = params[0].dataIndex;
          var row = rows[idx];
          return [
            "<strong>" + row.date + "</strong>",
            "组合总值：" + formatMoney(row.amount),
            "浮动盈亏：" + formatMoney(row.profit),
            "收益率：" + formatPercent(row.profitRatio, 2),
          ].join("<br>");
        },
      },
      legend: { top: 28, left: 8 },
      grid: { top: 72, right: 56, bottom: 36, left: 62, containLabel: true },
      xAxis: {
        type: "category",
        data: rows.map(function (row) { return row.date; }),
        boundaryGap: rows.length === 1,
      },
      yAxis: [
        {
          type: "value",
          name: "总值",
          axisLabel: { formatter: "{value}" },
          splitLine: { lineStyle: { color: "#eceff1" } },
          scale: true,
        },
        {
          type: "value",
          name: "收益率",
          axisLabel: { formatter: "{value}%" },
          splitLine: { show: false },
          scale: true,
        },
      ],
      series: [
        {
          name: "组合总值",
          type: "line",
          smooth: true,
          yAxisIndex: 0,
          data: rows.map(function (row) { return Number(row.amount || 0).toFixed(2); }),
        },
        {
          name: "收益率",
          type: "line",
          smooth: true,
          yAxisIndex: 1,
          data: rows.map(function (row) { return Number(row.profitRatio || 0).toFixed(2); }),
        },
      ],
    });
  }

  function renderCorrelation(data) {
    var correlation = data.correlation || {};
    var labels = correlation.labels || [];
    var points = correlation.points || [];
    if (labels.length < 2 || points.length === 0) {
      renderEmpty("fund-correlation-heatmap-chart", "近期基金净值数据不足，暂无法计算相关性");
      return;
    }

    var chart = initChart("fund-correlation-heatmap-chart");
    if (!chart) {
      return;
    }
    chart.clear();
    chart.setOption({
      tooltip: {
        position: "top",
        formatter: function (params) {
          return [
            labels[params.value[1]] + " / " + labels[params.value[0]],
            "相关性：" + Number(params.value[2] || 0).toFixed(2),
          ].join("<br>");
        },
      },
      grid: { top: 26, right: 24, bottom: 74, left: 92, containLabel: true },
      xAxis: {
        type: "category",
        data: labels,
        axisLabel: { rotate: 35, formatter: function (value) { return trimLabel(value, 10); } },
        splitArea: { show: true },
      },
      yAxis: {
        type: "category",
        data: labels,
        axisLabel: { formatter: function (value) { return trimLabel(value, 10); } },
        splitArea: { show: true },
      },
      visualMap: {
        min: -1,
        max: 1,
        calculable: true,
        orient: "horizontal",
        left: "center",
        bottom: 8,
        inRange: { color: ["#2563eb", "#f5f5f5", "#dc2626"] },
      },
      series: [
        {
          name: "相关性",
          type: "heatmap",
          data: points.map(function (point) {
            return [point.x, point.y, Number(point.value || 0).toFixed(4)];
          }),
          label: {
            show: true,
            formatter: function (params) {
              return Number(params.value[2] || 0).toFixed(2);
            },
          },
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowColor: "rgba(0, 0, 0, 0.18)",
            },
          },
        },
      ],
    });
  }

  function renderNAVTrend(data) {
    var trend = data.navTrend || {};
    var rows = trend.series || [];
    if (rows.length === 0) {
      renderEmpty("fund-nav-trend-chart", "近期基金净值缓存不足，刷新后生成波动曲线");
      return;
    }

    var chart = initChart("fund-nav-trend-chart");
    if (!chart) {
      return;
    }

    var datesMap = {};
    rows.forEach(function (row) {
      (row.points || []).forEach(function (point) {
        if (point.date) {
          datesMap[point.date] = true;
        }
      });
    });
    var dates = Object.keys(datesMap).sort();
    var series = rows.map(function (row) {
      var pointByDate = {};
      (row.points || []).forEach(function (point) {
        pointByDate[point.date] = point;
      });
      return {
        name: row.name || row.code,
        type: "line",
        smooth: true,
        showSymbol: false,
        symbolSize: 6,
        connectNulls: false,
        triggerLineEvent: true,
        emphasis: { focus: "series", lineStyle: { width: 4 } },
        data: dates.map(function (date) {
          var point = pointByDate[date];
          return point ? {
            value: Number(point.returnRatio || 0).toFixed(2),
            unitNav: Number(point.unitNav || 0).toFixed(4),
            date: date,
            fundName: row.name || row.code,
          } : null;
        }),
      };
    });

    chart.clear();
    chart.setOption({
      color: palette,
      title: {
        text: "近90日累计涨跌幅",
        subtext: "按各基金自身净值归一化，首个交易日为0%",
        left: 8,
        top: 0,
        textStyle: { fontSize: 14, fontWeight: 500 },
        subtextStyle: { color: "#607d8b" },
      },
      tooltip: {
        trigger: "item",
        triggerOn: "mousemove|click",
        formatter: function (params) {
          var point = params.data || {};
          return [
            "<strong>" + (point.fundName || params.seriesName) + "</strong>",
            "日期：" + (point.date || params.name || ""),
            "累计涨跌幅：" + formatPercent(point.value, 2),
            "单位净值：" + (point.unitNav || "--"),
          ].join("<br>");
        },
      },
      legend: {
        type: "scroll",
        top: 36,
        left: 8,
        right: 8,
        itemWidth: 10,
        itemHeight: 10,
      },
      grid: { top: 92, right: 34, bottom: 42, left: 58, containLabel: true },
      xAxis: {
        type: "category",
        data: dates,
        boundaryGap: false,
        axisLabel: { formatter: function (value) { return value.slice(5); } },
      },
      yAxis: {
        type: "value",
        name: "累计涨跌幅",
        axisLabel: { formatter: "{value}%" },
        splitLine: { lineStyle: { color: "#eceff1" } },
        scale: true,
      },
      series: series,
    });
  }

  function setCorrelationRefreshStatus(state, message) {
    var box = document.getElementById("fund-correlation-refresh-status");
    if (!box) {
      return;
    }
    var text = box.querySelector("[data-correlation-refresh-message]");
    var progress = box.querySelector(".progress");
    if (text) {
      text.textContent = message || "";
    }
    if (progress) {
      progress.hidden = state !== "loading";
    }
    box.hidden = !message;
    box.classList.toggle("is-error", state === "error");
    box.classList.toggle("is-done", state === "done");
  }

  function maybeRefreshCorrelation(data) {
    var refresh = data.correlationRefresh || {};
    if (!refresh.needed || !refresh.url || !window.fetch) {
      return;
    }
    setCorrelationRefreshStatus("loading", refresh.message || "正在刷新历史净值数据...");
    fetch(refresh.url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ funds: refresh.funds || [] }),
    }).then(function (resp) {
      if (!resp.ok) {
        throw new Error("HTTP " + resp.status);
      }
      return resp.json();
    }).then(function (payload) {
      if (payload.correlation) {
        data.correlation = payload.correlation;
        renderCorrelation(data);
      }
      if (payload.navTrend) {
        data.navTrend = payload.navTrend;
        renderNAVTrend(data);
      }
      resizeCharts();
      data.correlationRefresh = payload.refresh || {};
      var warnings = payload.warnings || [];
      if (warnings.length > 0) {
        setCorrelationRefreshStatus("error", warnings[0]);
        return;
      }
      setCorrelationRefreshStatus("done", "历史净值数据已更新。");
      window.setTimeout(function () {
        setCorrelationRefreshStatus("done", "");
      }, 2800);
    }).catch(function (err) {
      setCorrelationRefreshStatus("error", "历史净值刷新失败，继续使用本地缓存：" + err.message);
    });
  }

  function renderComparison(data) {
    var rows = (data.comparisons || []);
    var metrics = (data.comparisonMetrics || []);
    if (rows.length === 0 || metrics.length === 0) {
      renderEmpty("fund-comparison-radar-chart", "暂无可用于替代基金对比的数据");
      return;
    }

    var chart = initChart("fund-comparison-radar-chart");
    if (!chart) {
      return;
    }
    chart.setOption({
      color: palette,
      tooltip: {
        trigger: "item",
        formatter: function (params) {
          var row = rows[params.dataIndex];
          var lines = ["<strong>" + row.name + "</strong>", "状态：" + row.status];
          metrics.forEach(function (metric, idx) {
            lines.push(metric.name + "：" + Number(row.values[idx] || 0).toFixed(1));
          });
          return lines.join("<br>");
        },
      },
      legend: {
        type: "scroll",
        bottom: 0,
        left: 8,
        right: 8,
      },
      radar: {
        center: ["50%", "48%"],
        radius: "64%",
        indicator: metrics.map(function (metric) {
          return { name: metric.name, max: metric.max || 100 };
        }),
      },
      series: [
        {
          name: "基金对比",
          type: "radar",
          data: rows.map(function (row) {
            return {
              name: row.name,
              value: row.values,
            };
          }),
          areaStyle: { opacity: 0.06 },
          lineStyle: { width: 2 },
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
      chartIds.forEach(showUnavailable);
      return;
    }

    renderThemeExposure(data);
    renderStockExposure(data);
    renderRiskReturn(data);
    renderHistory(data);
    renderNAVTrend(data);
    renderCorrelation(data);
    maybeRefreshCorrelation(data);
    renderComparison(data);
    window.fundPortfolioResizeCharts = resizeCharts;
    window.addEventListener("resize", resizeCharts);
  });
})();
