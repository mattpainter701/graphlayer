(() => {
  'use strict';

  const pages = new Set(['workspace', 'imports', 'investigation', 'topology']);
  const aliases = {
    '': { page: 'workspace' },
    workspace: { page: 'workspace' },
    'workspace-home': { page: 'workspace' },
    imports: { page: 'imports' },
    investigation: { page: 'investigation' },
    topology: { page: 'topology' },
    'network-path': { page: 'topology' },
    'policy-inventory': { page: 'investigation', anchor: 'policy-inventory' },
    objects: { page: 'investigation', anchor: 'objects' }
  };
  const titles = {
    workspace: 'Workspace home',
    imports: 'Sources & imports',
    investigation: 'Firewall investigation',
    topology: 'Network topology'
  };
  function routeFromHash(hash) {
    const raw = String(hash || '').replace(/^#/, '');
    const [rawRoutePart, hashAnchor] = raw.split('#', 2);
    const routePart = (rawRoutePart || '').split('?', 1)[0];
    let routeName = routePart;
    try { routeName = decodeURIComponent(routePart); } catch (_) { /* Keep malformed paths on the workspace route. */ }
    const route = aliases[routeName] || { page: 'workspace' };
    return { page: route.page, anchor: hashAnchor || route.anchor || null };
  }

  function hashFor(page, anchor) {
    if (anchor && (anchor === 'policy-inventory' || anchor === 'objects')) return `#${anchor}`;
    return `#${page}`;
  }

  function init() {
    const main = document.querySelector('.main');
    if (!main) return;
    const pageNodes = Array.from(main.querySelectorAll('[data-app-page]'));
    if (!pageNodes.length) return;

    let activePage = null;
    let activeAnchor = null;
    let anchorFrame = 0;

    function showRoute(route) {
      const page = pages.has(route.page) ? route.page : 'workspace';
      pageNodes.forEach(node => {
        const active = node.dataset.appPage === page;
        node.hidden = !active;
        node.classList.toggle('is-active-page', active);
        if (active) node.setAttribute('aria-current', 'page');
        else node.removeAttribute('aria-current');
      });

      document.querySelectorAll('.nav-list a.nav-item').forEach(link => {
        const parsed = routeFromHash(link.hash);
        const active = parsed.page === page && (route.anchor ? parsed.anchor === route.anchor : !parsed.anchor);
        link.classList.toggle('active', active);
        if (active) link.setAttribute('aria-current', 'page');
        else link.removeAttribute('aria-current');
      });

      document.title = `GraphLayer · ${titles[page]}`;

      if (anchorFrame) cancelAnimationFrame(anchorFrame);
      if (route.anchor) {
        anchorFrame = requestAnimationFrame(() => {
          const target = document.getElementById(route.anchor);
          if (target && target.closest('[data-app-page]')?.dataset.appPage === page) {
            target.scrollIntoView({ block: 'start', behavior: 'smooth' });
          }
        });
      }

      const anchor = route.anchor || null;
      const changed = activePage !== page;
      const routeChanged = changed || activeAnchor !== anchor;
      activePage = page;
      activeAnchor = anchor;
      if (changed && !anchor) window.scrollTo({ top: 0, behavior: 'instant' });
      if (routeChanged) {
        document.dispatchEvent(new CustomEvent('graphlayer:page-changed', {
          detail: { page, anchor: anchor || undefined, changed }
        }));
      }
    }

    function navigate(page, anchor) {
      const route = aliases[page] || { page };
      const targetPage = pages.has(route.page) ? route.page : 'workspace';
      const targetAnchor = anchor || route.anchor || null;
      const nextHash = hashFor(targetPage, targetAnchor);
      showRoute({ page: targetPage, anchor: targetAnchor });
      // Route names share some legacy element IDs. Avoid native anchor jumps.
      if (location.hash !== nextHash) history.pushState(null, '', nextHash);
    }

    document.addEventListener('graphlayer:navigate', event => {
      const detail = event.detail || {};
      if (!detail.page) return;
      navigate(detail.page, detail.anchor);
    });

    document.addEventListener('click', event => {
      const cta = event.target.closest('[data-nav-page]');
      if (cta) {
        event.preventDefault();
        navigate(cta.dataset.navPage, cta.dataset.navAnchor);
        return;
      }
      const link = event.target.closest('.nav-list a.nav-item');
      if (link) {
        const route = routeFromHash(link.hash);
        event.preventDefault();
        navigate(route.page, route.anchor);
      }
    });

    window.addEventListener('hashchange', () => showRoute(routeFromHash(location.hash)));
    showRoute(routeFromHash(location.hash));
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init, { once: true });
  else init();
})();
