(function () {
  "use strict";

  var openedAtKey = "investool_tiantian_login_opened_at";

  function onReady(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
      return;
    }
    fn();
  }

  function setStatus(text, active) {
    var status = document.getElementById("tiantian-login-status");
    if (!status) {
      return;
    }
    status.textContent = text || status.getAttribute("data-default-text") || "";
    status.classList.toggle("is-active", !!active);
  }

  function markOpened() {
    try {
      window.localStorage.setItem(openedAtKey, String(Date.now()));
    } catch (err) {
      // localStorage can be disabled; the flow still works without the visual state.
    }
    setStatus("天天基金官网已打开，登录完成后点击继续导入", true);
  }

  function restoreOpenedState() {
    try {
      if (window.localStorage.getItem(openedAtKey)) {
        setStatus("已打开过天天基金官网，登录完成后可继续导入", true);
      }
    } catch (err) {
      setStatus("", false);
    }
  }

  onReady(function () {
    var openLink = document.getElementById("tiantian-open-link");
    if (openLink) {
      openLink.addEventListener("click", markOpened);
    }
    restoreOpenedState();
  });
})();
