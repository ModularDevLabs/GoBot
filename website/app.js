(function () {
  const root = document.documentElement;
  const toggle = document.getElementById('themeToggle');
  const yearEl = document.getElementById('year');
  const key = 'fundamentum-theme';

  const preferred = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  const theme = localStorage.getItem(key) || preferred;
  root.setAttribute('data-theme', theme);

  function syncThemeLabel() {
    if (!toggle) return;
    const dark = root.getAttribute('data-theme') === 'dark';
    toggle.textContent = dark ? 'Light mode' : 'Dark mode';
  }

  syncThemeLabel();

  if (toggle) {
    toggle.addEventListener('click', function () {
      const next = root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
      root.setAttribute('data-theme', next);
      localStorage.setItem(key, next);
      syncThemeLabel();
    });
  }

  if (yearEl) yearEl.textContent = String(new Date().getFullYear());

  const items = Array.prototype.slice.call(document.querySelectorAll('.reveal'));
  if ('IntersectionObserver' in window) {
    const io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('show');
          io.unobserve(entry.target);
        }
      });
    }, { threshold: 0.14 });
    items.forEach(function (el) { io.observe(el); });
  } else {
    items.forEach(function (el) { el.classList.add('show'); });
  }
})();
