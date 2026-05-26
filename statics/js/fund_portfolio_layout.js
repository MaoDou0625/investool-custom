(function () {
  "use strict";

  function onReady(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
      return;
    }
    fn();
  }

  function targetFromLink(link) {
    return link.getAttribute("data-portfolio-nav") || String(link.getAttribute("href") || "").replace(/^#/, "");
  }

  function currentHashTarget() {
    return String(window.location.hash || "").replace(/^#/, "");
  }

  function resizePortfolioCharts() {
    if (typeof window.fundPortfolioResizeCharts === "function") {
      window.fundPortfolioResizeCharts();
      window.setTimeout(window.fundPortfolioResizeCharts, 80);
      return;
    }
    window.dispatchEvent(new Event("resize"));
  }

  function centerActiveLink(link) {
    if (!link) {
      return;
    }
    var nav = link.closest(".fund-portfolio-side-nav");
    if (!nav) {
      return;
    }
    nav.scrollLeft = link.offsetLeft - (nav.clientWidth - link.offsetWidth) / 2;
  }

  onReady(function () {
    var page = document.getElementById("fund_portfolio_content");
    if (!page) {
      return;
    }

    var links = Array.prototype.slice.call(document.querySelectorAll(".fund-portfolio-side-nav a[href^='#']"));
    var panels = Array.prototype.slice.call(document.querySelectorAll(".fund-portfolio-section[data-portfolio-page]"));
    if (links.length === 0 || panels.length === 0) {
      return;
    }

    var shell = page.querySelector(".fund-portfolio-shell");
    var availableTargets = links.map(targetFromLink);
    var defaultTarget = availableTargets[0];

    function hasTarget(target) {
      return availableTargets.indexOf(target) !== -1 && panels.some(function (panel) {
        return panel.getAttribute("data-portfolio-page") === target;
      });
    }

    function activate(target, options) {
      var settings = options || {};
      var activeTarget = hasTarget(target) ? target : defaultTarget;
      var activeLink = null;

      links.forEach(function (link) {
        var isActive = targetFromLink(link) === activeTarget;
        link.classList.toggle("is-active", isActive);
        if (isActive) {
          activeLink = link;
          link.setAttribute("aria-current", "page");
        } else {
          link.removeAttribute("aria-current");
        }
      });

      panels.forEach(function (panel) {
        var isActive = panel.getAttribute("data-portfolio-page") === activeTarget;
        panel.hidden = !isActive;
        panel.setAttribute("aria-hidden", isActive ? "false" : "true");
      });

      page.setAttribute("data-active-portfolio-page", activeTarget);
      if (settings.updateHash !== false && window.history && window.history.replaceState) {
        window.history.replaceState(null, "", "#" + activeTarget);
      }

      if (settings.scroll !== false && shell) {
        window.requestAnimationFrame(function () {
          shell.scrollIntoView({ block: "start" });
          centerActiveLink(activeLink);
        });
      } else if (activeLink) {
        window.requestAnimationFrame(function () { centerActiveLink(activeLink); });
      }

      if (activeTarget === "portfolio-analysis") {
        window.requestAnimationFrame(resizePortfolioCharts);
      }
    }

    links.forEach(function (link) {
      link.addEventListener("click", function (event) {
        event.preventDefault();
        activate(targetFromLink(link), { scroll: true, updateHash: true });
      });
    });

    window.addEventListener("hashchange", function () {
      activate(currentHashTarget(), { scroll: true, updateHash: false });
    });

    activate(currentHashTarget(), { scroll: currentHashTarget() !== "", updateHash: false });
  });
})();
