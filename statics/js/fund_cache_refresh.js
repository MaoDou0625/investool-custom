(function () {
  function text(id, value) {
    var el = document.getElementById(id);
    if (el) {
      el.textContent = value;
    }
  }

  function setProgress(width) {
    var el = document.getElementById("fund-cache-progress-bar");
    if (el) {
      el.style.width = Math.max(0, Math.min(100, width || 0)) + "%";
    }
  }

  function progressDetail(data) {
    if (data.error) {
      return data.error;
    }
    if (data.refreshing) {
      if (!data.total) {
        return data.stage === "build_plan" ? "正在按 4433、缺失缓存和过期时间生成优先刷新计划。" : "正在获取基金代码列表，稍后会开始补全基金详情。";
      }
      var prefix = data.mode === "priority" ? "优先刷新" : "全量刷新";
      var detail = prefix + " " + data.processed + "/" + data.total + "，成功 " + data.succeeded + "，失败 " + data.failed + "。";
      if (data.mode === "priority") {
        detail += " 本批 4433 过期 " + (data.priority_4433_count || 0) + " 只，缺失缓存 " + (data.missing_count || 0) + " 只，其他过期 " + (data.stale_other_count || 0) + " 只。";
        if (data.deferred_count) {
          detail += " 另有 " + data.deferred_count + " 只排到后续空闲批次。";
        }
      }
      return detail;
    }
    if (data.stage === "done") {
      var doneText = "刷新完成:" + data.finished_at + "，缓存 " + data.fund_count + " 只基金，严格 4433 " + data.fund_4433_count + " 只，每日候选 " + data.recommendation_count + " 只。";
      if (data.mode === "priority") {
        doneText += " 最近一批按优先级刷新 " + (data.planned || data.total || 0) + " 只。";
      }
      return doneText;
    }
    return "当前缓存状态会在刷新时自动更新。";
  }

  function updateLink(panel, data) {
    var link = document.getElementById("fund-cache-refresh-link");
    if (!link) {
      return;
    }
    if (data.refreshing) {
      link.classList.add("disabled");
      link.innerHTML = '<i class="material-icons left">hourglass_empty</i>刷新中';
      return;
    }
    link.classList.remove("disabled");
    if (data.stage === "done") {
      link.href = panel.getAttribute("data-page-url") || "/fund#4433";
      link.innerHTML = '<i class="material-icons left">refresh</i>刷新页面查看新列表';
      return;
    }
    link.innerHTML = '<i class="material-icons left">sync</i>刷新本地缓存';
  }

  function render(panel, data) {
    panel.classList.toggle("is-refreshing", !!data.refreshing);
    text("fund-cache-refresh-title", data.refreshing || data.error ? data.stage_text : data.fund_count + " 只基金");
    text("fund-cache-refresh-count", data.fund_count || 0);
    text("fund-cache-refresh-4433", data.fund_4433_count || 0);
    text("fund-cache-refresh-recommendation", data.recommendation_count || 0);
    text("fund-cache-refresh-stage", data.stage_text || "尚未刷新");
    text("fund-cache-refresh-time", "更新时间:" + (data.updated_at || "--"));
    text("fund-cache-progress-detail", progressDetail(data));
    setProgress(data.percent || 0);
    updateLink(panel, data);
  }

  function poll(panel) {
    var statusURL = panel.getAttribute("data-status-url");
    if (!statusURL || !window.jQuery) {
      return;
    }
    window.jQuery.getJSON(statusURL, function (data) {
      render(panel, data);
      if (data.refreshing) {
        window.setTimeout(function () {
          poll(panel);
        }, 2500);
      }
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    var panel = document.getElementById("fund-cache-refresh-status");
    if (!panel) {
      return;
    }
    poll(panel);
  });
})();
