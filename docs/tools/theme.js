/* 在线工具站主题：跟随系统 + 手动切换
 *
 * 用法（在 <head> 里、样式表之后引入，尽早执行以避免闪烁）：
 *   <script src="theme.js"></script>
 *
 * 优先级：localStorage 里的用户选择 > 系统 prefers-color-scheme。
 * 页面自己的切换按钮调用 SiteTheme.toggle() 即可，
 * 也可注册 SiteTheme.onChange(fn) 监听变化（例如 Markdown 页要同步代码高亮样式表）。
 */
(function () {
  "use strict";

  var KEY = "site.theme";
  var listeners = [];

  function stored() {
    try {
      var v = localStorage.getItem(KEY);
      return v === "light" || v === "dark" ? v : null;
    } catch (e) {
      return null;
    }
  }

  function systemTheme() {
    return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function current() {
    return document.documentElement.getAttribute("data-theme") || systemTheme();
  }

  function apply(theme, persist) {
    var t = theme === "light" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", t);
    if (persist) {
      try {
        localStorage.setItem(KEY, t);
      } catch (e) {
        /* 隐私模式下 localStorage 不可用：忽略，仅本次会话生效 */
      }
    }
    for (var i = 0; i < listeners.length; i++) {
      try {
        listeners[i](t);
      } catch (e) {
        /* 单个监听器出错不应影响其它监听器 */
      }
    }
    return t;
  }

  function toggle() {
    return apply(current() === "dark" ? "light" : "dark", true);
  }

  // 首帧之前写入 data-theme：脚本放在 <head> 里即可避免深色闪白。
  apply(stored() || systemTheme(), false);

  // 用户没有手动选择过时，跟随系统切换。
  if (window.matchMedia) {
    var mq = window.matchMedia("(prefers-color-scheme: dark)");
    var onChange = function () {
      if (!stored()) apply(systemTheme(), false);
    };
    if (mq.addEventListener) mq.addEventListener("change", onChange);
    else if (mq.addListener) mq.addListener(onChange);
  }

  window.SiteTheme = {
    get: current,
    set: function (theme) {
      return apply(theme, true);
    },
    toggle: toggle,
    onChange: function (fn) {
      if (typeof fn === "function") listeners.push(fn);
    },
  };
})();
