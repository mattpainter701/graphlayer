(() => {
  'use strict';

  const NS = 'http://www.w3.org/2000/svg';
  const tags = new Set(['svg', 'g', 'defs', 'marker', 'path', 'line', 'polyline', 'polygon', 'rect', 'circle', 'ellipse', 'text', 'tspan', 'title', 'desc', 'linearGradient', 'radialGradient', 'stop', 'clipPath', 'mask', 'pattern', 'use']);
  const properties = ['display', 'visibility', 'opacity', 'fill', 'fill-opacity', 'fill-rule', 'stroke', 'stroke-width', 'stroke-opacity', 'stroke-linecap', 'stroke-linejoin', 'stroke-dasharray', 'stroke-dashoffset', 'font-family', 'font-size', 'font-weight', 'font-style', 'letter-spacing', 'text-anchor', 'dominant-baseline', 'paint-order', 'vector-effect', 'marker-start', 'marker-mid', 'marker-end', 'clip-path'];

  function localReference(value) {
    if (!value.includes('url(')) return value;
    const match = /^url\(["']?([^"')]+)["']?\)$/.exec(value);
    if (!match) return '';
    const reference = match[1];
    if (/^#[A-Za-z_][\w:.-]*$/.test(reference)) return `url(${reference})`;
    const hash = reference.lastIndexOf('#');
    if (hash < 0 || reference.slice(0, hash) !== document.URL.split('#')[0]) return '';
    const fragment = reference.slice(hash);
    return /^#[A-Za-z_][\w:.-]*$/.test(fragment) ? `url(${fragment})` : '';
  }

  function standalone(svg, options) {
    if (!svg || svg.namespaceURI !== NS || svg.localName !== 'svg' || !svg.isConnected) throw new Error('A rendered graph is required for SVG export.');
    const bounds = svg.getBoundingClientRect();
    const view = svg.viewBox.baseVal;
    if (!(bounds.width > 0 && bounds.height > 0 && view.width > 0 && view.height > 0)) throw new Error('The graph must be visible before exporting.');
    const copy = svg.cloneNode(true);
    const originals = [svg, ...svg.querySelectorAll('*')];
    const copies = [copy, ...copy.querySelectorAll('*')];
    originals.forEach((original, index) => {
      const target = copies[index];
      if (!tags.has(original.localName) || original.namespaceURI !== NS) { target.remove(); return; }
      const computed = getComputedStyle(original);
      target.removeAttribute('style');
      for (const attribute of [...target.attributes]) {
        const name = attribute.localName.toLowerCase();
        if (name.startsWith('on') || name.startsWith('data-') || name === 'tabindex' || name === 'role' || name.startsWith('aria-')) target.removeAttributeNode(attribute);
        else if (name === 'href' && !/^#[A-Za-z_][\w:.-]*$/.test(attribute.value)) target.removeAttributeNode(attribute);
        else if (attribute.value.includes('url(')) {
          const safe = localReference(attribute.value);
          if (safe) target.setAttribute(attribute.name, safe); else target.removeAttributeNode(attribute);
        }
      }
      properties.forEach((property) => {
        const value = localReference(computed.getPropertyValue(property));
        if (value) target.style.setProperty(property, value);
      });
    });
    // Labels and metadata are text, never markup or a copy of the source workspace.
    copy.querySelectorAll(':scope > title, :scope > desc').forEach((element) => element.remove());
    const title = document.createElementNS(NS, 'title');
    title.textContent = String(options.title || 'GraphLayer graph').slice(0, 240);
    const description = document.createElementNS(NS, 'desc');
    description.textContent = String(options.description || 'Configured topology view; live observations are unavailable.').slice(0, 6000);
    copy.prepend(title, description);
    copy.setAttribute('xmlns', NS);
    copy.setAttribute('width', String(Math.round(bounds.width)));
    copy.setAttribute('height', String(Math.round(bounds.height)));
    copy.setAttribute('role', 'img');
    copy.setAttribute('aria-label', title.textContent);
    copy.style.background = '#f8fafc';
    return `<?xml version="1.0" encoding="UTF-8"?>\n${new XMLSerializer().serializeToString(copy)}\n`;
  }

  function download(svg, options = {}) {
    const filename = /^[A-Za-z0-9][A-Za-z0-9._-]*\.svg$/.test(options.filename || '') ? options.filename : 'graphlayer-graph.svg';
    const content = standalone(svg, options);
    const url = URL.createObjectURL(new Blob([content], { type: 'image/svg+xml;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.hidden = true;
    document.body.append(link);
    try { link.click(); } finally { link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000); }
  }

  window.GraphLayerGraphExport = Object.freeze({ download });
})();
