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

  const modal = document.getElementById('shotModal');
  const modalImg = document.getElementById('shotModalImage');
  const modalCap = document.getElementById('shotModalCaption');
  const closeBtn = document.getElementById('shotClose');
  const openers = Array.prototype.slice.call(document.querySelectorAll('.shot-open'));

  function closeModal() {
    if (!modal) return;
    modal.classList.remove('show');
    modal.setAttribute('aria-hidden', 'true');
    if (modalImg) modalImg.src = '';
  }

  openers.forEach(function (btn) {
    btn.addEventListener('click', function () {
      if (!modal || !modalImg || !modalCap) return;
      const src = btn.getAttribute('data-full') || '';
      const title = btn.getAttribute('data-title') || 'Screenshot';
      modalImg.src = src;
      modalImg.alt = title;
      modalCap.textContent = title;
      modal.classList.add('show');
      modal.setAttribute('aria-hidden', 'false');
    });
  });

  if (closeBtn) closeBtn.addEventListener('click', closeModal);
  if (modal) {
    modal.addEventListener('click', function (ev) {
      if (ev.target === modal) closeModal();
    });
  }
  document.addEventListener('keydown', function (ev) {
    if (ev.key === 'Escape') closeModal();
  });

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
