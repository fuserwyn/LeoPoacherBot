// Эмуляция Telegram.WebApp для окна с мини-аппом.
//
// SPA берёт токен из window.Telegram.WebApp.initData (bot/webapp.py), поэтому
// вместо подписанного initData отдаём туда токен десктопной сессии — сервер
// различает их по префиксу. Остальные методы закрывают то, что страница реально
// вызывает: ready/expand, safe area, viewport, HapticFeedback, showAlert, openLink.
//
// Скрипт выполняется до загрузки страницы, поэтому `const tg = window.Telegram?.WebApp`
// подхватывает объект синхронно.
(function () {
  try { window.__MYVIBELAB_DESKTOP__ = true; } catch (e) {}
  if (window.Telegram && window.Telegram.WebApp) return;

  var INIT_DATA = __MVL_INIT_DATA__;
  var USER = __MVL_USER__;
  var noop = function () {};
  var listeners = {};

  function emit(name) {
    (listeners[name] || []).forEach(function (cb) {
      try { cb(); } catch (e) {}
    });
  }

  // Чужой origin ловит Rust-хук on_navigation: открывает ссылку в системе и
  // отменяет переход, поэтому состояние SPA не теряется.
  function openExternally(url) {
    if (!url) return;
    try { window.location.href = url; } catch (e) {}
  }

  var WebApp = {
    initData: INIT_DATA,
    initDataUnsafe: { user: USER },
    version: '8.0',
    platform: __MVL_PLATFORM__,
    colorScheme: window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
    isExpanded: true,
    safeAreaInset: { top: 0, bottom: 0, left: 0, right: 0 },
    contentSafeAreaInset: { top: 0, bottom: 0, left: 0, right: 0 },

    get viewportHeight() { return window.innerHeight; },
    get viewportStableHeight() { return window.innerHeight; },

    isVersionAtLeast: function () { return true; },
    ready: noop,
    expand: noop,
    close: noop,
    requestFullscreen: noop,
    exitFullscreen: noop,
    disableVerticalSwipes: noop,
    enableVerticalSwipes: noop,
    setHeaderColor: noop,
    setBackgroundColor: noop,

    onEvent: function (name, cb) {
      if (typeof cb !== 'function') return;
      (listeners[name] = listeners[name] || []).push(cb);
    },
    offEvent: function (name, cb) {
      var list = listeners[name] || [];
      var i = list.indexOf(cb);
      if (i >= 0) list.splice(i, 1);
    },

    showAlert: function (msg, cb) { window.alert(msg); if (cb) cb(); },
    showConfirm: function (msg, cb) { var ok = window.confirm(msg); if (cb) cb(ok); },
    openLink: openExternally,
    openTelegramLink: openExternally,

    // openInvoice намеренно не определяем: Stars платятся только внутри Telegram.
    // Без этого метода мини-апп сам уходит в ветку «оплати по ссылке», и ссылка
    // открывается в системном Telegram/браузере.

    HapticFeedback: {
      impactOccurred: noop,
      notificationOccurred: noop,
      selectionChanged: noop
    },
    MainButton: {
      show: noop, hide: noop, enable: noop, disable: noop,
      setText: noop, setParams: noop, onClick: noop, offClick: noop,
      showProgress: noop, hideProgress: noop
    },
    BackButton: { show: noop, hide: noop, onClick: noop, offClick: noop }
  };

  window.addEventListener('resize', function () {
    emit('viewportChanged');
  });
  if (window.matchMedia) {
    var dark = window.matchMedia('(prefers-color-scheme: dark)');
    var onScheme = function () {
      WebApp.colorScheme = dark.matches ? 'dark' : 'light';
      emit('themeChanged');
    };
    if (dark.addEventListener) dark.addEventListener('change', onScheme);
  }

  // Страница мини-аппа грузит официальный telegram-web-app.js, а он безусловно
  // делает `window.Telegram = {}` и кладёт туда свой WebApp. Вне Telegram у того
  // initData пустой, и мост со скриптом входа просто затирался — boot() не видел
  // токена и молча вставал, не сделав ни одного запроса к API.
  // Поэтому отдаём мост через геттеры, а присваивания игнорируем. Сеттеры (а не
  // writable:false) нужны, чтобы чужой скрипт не падал в strict mode.
  var holder = {};
  Object.defineProperty(holder, 'WebApp', {
    configurable: false,
    enumerable: true,
    get: function () { return WebApp; },
    set: function () {}
  });
  Object.defineProperty(window, 'Telegram', {
    configurable: false,
    enumerable: true,
    get: function () { return holder; },
    set: function () {}
  });
})();
