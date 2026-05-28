(function () {
  var sessionKey = "investool_launcher_session";
  var params = new URLSearchParams(window.location.search);
  var session = params.get("launcher_session");

  if (session) {
    window.sessionStorage.setItem(sessionKey, session);
  } else {
    session = window.sessionStorage.getItem(sessionKey);
  }
  if (!session) {
    return;
  }

  function heartbeat() {
    var endpoint = "/launcher/session/heartbeat?session=" + encodeURIComponent(session);
    if (navigator.sendBeacon) {
      navigator.sendBeacon(endpoint);
      return;
    }
    window.fetch(endpoint, {
      method: "POST",
      keepalive: true,
      credentials: "same-origin"
    }).catch(function () {});
  }

  heartbeat();
  window.setInterval(heartbeat, 3000);
  window.addEventListener("pagehide", heartbeat);
})();
