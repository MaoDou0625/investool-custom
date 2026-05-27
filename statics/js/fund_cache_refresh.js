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
        return "正在获取基金代码列表，稍后会开始补全基金详情。";
      }
      return "已处理 " + data.processed + "/" + data.total + "，成功 " + data.succeeded + "，失败 " + data.failed + "。";
    }
    if (data.stage === "done") {
      return "刷新完成:" + data.finished_at + "，缓存 " + data.fund_count + " 只基金，严格 4433 " + data.fund_4433_count + " 只，每日候选 " + data.recommendation_count + " 只。";
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
