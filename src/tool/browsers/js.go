package browsers

const InspectorJS = `function(cfg){
  cfg = cfg || {};
  function txt(v, n){ v = String(v || '').replace(/\s+/g, ' ').trim(); if(!v) return ''; return n && v.length > n ? v.slice(0, n) + '...' : v; }
  function vis(el){ if(!el || !el.isConnected) return false; if(el.hidden || el.getAttribute('aria-hidden') === 'true') return false; var s = getComputedStyle(el); if(s.display === 'none' || s.visibility === 'hidden' || s.opacity === '0') return false; var r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0; }
  function esc(v){ if(window.CSS && CSS.escape) return CSS.escape(v); return String(v).replace(/([^a-zA-Z0-9_-])/g, '\\$1'); }
  function uniq(sel){ try { return document.querySelectorAll(sel).length === 1; } catch (e) { return false; } }
  function nth(el){ var i = 1, p = el; while((p = p.previousElementSibling) != null){ if(p.tagName === el.tagName) i++; } return i; }
  function path(el){ var parts = [], cur = el, depth = 0; while(cur && cur.nodeType === 1 && cur !== document.body && depth < 4){ var tag = cur.tagName.toLowerCase(); if(cur.id){ parts.unshift(tag + '#' + esc(cur.id)); return parts.join(' > '); } if(cur.getAttribute('name')){ parts.unshift(tag + '[name="' + String(cur.getAttribute('name')).replace(/"/g, '\\"') + '"]'); return parts.join(' > '); } parts.unshift(tag + ':nth-of-type(' + nth(cur) + ')'); cur = cur.parentElement; depth++; } return parts.length ? parts.join(' > ') : el.tagName.toLowerCase(); }
  function sel(el){ if(!el) return ''; var tag = el.tagName.toLowerCase(); if(el.id){ var s = '#' + esc(el.id); if(uniq(s)) return s; } var attrs = ['name', 'placeholder', 'type', 'data-testid']; for(var i=0;i<attrs.length;i++){ var value = el.getAttribute(attrs[i]); if(!value) continue; var s2 = tag + '[' + attrs[i] + '="' + String(value).replace(/"/g, '\\"') + '"]'; if(uniq(s2)) return s2; } return path(el); }
  function safeValue(el){
    if(!el) return '';
    var tag = (el.tagName || '').toLowerCase();
    var type = String(el.type || '').toLowerCase();
    if(tag === 'input' || tag === 'textarea' || tag === 'select'){
      if(type === 'password') return el.value ? '[masked]' : '';
      return el.value ? '[filled]' : '';
    }
    return txt(el.innerText || el.textContent || '', 80);
  }
  function uniqList(items, limit){ var out = [], seen = {}; for(var i=0;i<items.length;i++){ var v = txt(items[i], 120); if(!v || seen[v]) continue; seen[v] = 1; out.push(v); if(limit && out.length >= limit) break; } return out; }
  function labelFor(el){
    if(!el) return '';
    var aria = el.getAttribute('aria-label');
    if(aria) return txt(aria, 60);
    if(el.labels && el.labels.length){
      for(var i=0;i<el.labels.length;i++){
        var labelText = txt(el.labels[i].innerText || el.labels[i].textContent || '', 60);
        if(labelText) return labelText;
      }
    }
    if(el.id){
      try {
        var labelNode = document.querySelector('label[for="' + String(el.id).replace(/"/g, '\\"') + '"]');
        if(labelNode){
          var labelText2 = txt(labelNode.innerText || labelNode.textContent || '', 60);
          if(labelText2) return labelText2;
        }
      } catch (e) {}
    }
    var prev = el.previousElementSibling;
    if(prev){
      var prevText = txt(prev.innerText || prev.textContent || '', 60);
      if(prevText && prevText.length <= 60) return prevText;
    }
    return txt(el.getAttribute('placeholder') || el.name || el.id || el.type || el.tagName.toLowerCase(), 60);
  }
  function texts(selector, limit){
    var nodes;
    try { nodes = document.querySelectorAll(selector); } catch (e) { return []; }
    var out = [];
    for(var i=0;i<nodes.length;i++){
      if(!vis(nodes[i])) continue;
      var value = txt(nodes[i].innerText || nodes[i].textContent || '', 160);
      if(!value) continue;
      out.push(value);
      if(limit && out.length >= limit) break;
    }
    return uniqList(out, limit || 10);
  }
  function headingText(root){
    if(!root) return '';
    var nodes = root.querySelectorAll('h1,h2,h3,legend,.card-title,.form-title,.panel-title');
    for(var i=0;i<nodes.length;i++){
      if(!vis(nodes[i])) continue;
      var value = txt(nodes[i].innerText || nodes[i].textContent || '', 80);
      if(value) return value;
    }
    return '';
  }
  function summarizeForm(root){
    if(!root) return null;
    var fields = [];
    var nodes = root.querySelectorAll('input,textarea,select');
    for(var i=0;i<nodes.length;i++){
      var el = nodes[i];
      if(!vis(el)) continue;
      var valueState = safeValue(el);
      fields.push({ selector: sel(el), label: labelFor(el), name: txt(el.name || el.id || '', 40), type: txt(el.type || el.tagName.toLowerCase(), 20), placeholder: txt(el.getAttribute('placeholder') || '', 60), required: !!el.required, filled: valueState === '[filled]' || valueState === '[masked]' });
      if(fields.length >= 10) break;
    }
    var submitNodes = root.querySelectorAll('button,input[type="submit"],input[type="button"]');
    var submitTexts = [];
    for(var j=0;j<submitNodes.length;j++){
      if(!vis(submitNodes[j])) continue;
      submitTexts.push(txt(submitNodes[j].innerText || submitNodes[j].textContent || submitNodes[j].value || '', 60));
      if(submitTexts.length >= 4) break;
    }
    submitTexts = uniqList(submitTexts, 4);
    var title = headingText(root.closest('form,.card,.panel,[class*="card"],[class*="panel"]') || root);
    return { selector: sel(root), title: title, method: txt((root.getAttribute('method') || '').toLowerCase(), 16), action: txt(root.getAttribute('action') || '', 120), field_count: fields.length, submit_texts: submitTexts, fields: fields };
  }
  function syntheticForm(){
    var fields = [];
    var nodes = document.querySelectorAll('input,textarea,select');
    for(var i=0;i<nodes.length;i++){
      var el = nodes[i];
      if(!vis(el)) continue;
      var valueState = safeValue(el);
      fields.push({ selector: sel(el), label: labelFor(el), name: txt(el.name || el.id || '', 40), type: txt(el.type || el.tagName.toLowerCase(), 20), placeholder: txt(el.getAttribute('placeholder') || '', 60), required: !!el.required, filled: valueState === '[filled]' || valueState === '[masked]' });
      if(fields.length >= 8) break;
    }
    var submitNodes = document.querySelectorAll('button,input[type="submit"],input[type="button"]');
    var submitTexts = [];
    for(var j=0;j<submitNodes.length;j++){
      if(!vis(submitNodes[j])) continue;
      submitTexts.push(txt(submitNodes[j].innerText || submitNodes[j].textContent || submitNodes[j].value || '', 60));
      if(submitTexts.length >= 4) break;
    }
    submitTexts = uniqList(submitTexts, 4);
    if(fields.length === 0 && submitTexts.length === 0) return null;
    var title = texts('h1,h2,h3,legend,.card-title,.form-title,.panel-title', 3)[0] || '';
    return { selector: '(synthetic)', title: title, field_count: fields.length, submit_texts: submitTexts, fields: fields };
  }
  function treeRole(el){
    if(!el || !el.tagName) return '';
    var ar = txt(el.getAttribute && el.getAttribute('role') || '', 20).toLowerCase();
    if(ar) return ar;
    var tag = el.tagName.toUpperCase();
    if(tag === 'INPUT'){
      var type = String(el.type || 'text').toLowerCase();
      if(type === 'checkbox') return 'checkbox';
      if(type === 'radio') return 'radio';
      if(type === 'range') return 'slider';
      if(type === 'submit' || type === 'button' || type === 'reset') return 'button';
      return 'textbox';
    }
    var map = { A:'link', BUTTON:'button', SELECT:'combobox', TEXTAREA:'textbox', IMG:'img', MAIN:'main', NAV:'navigation', HEADER:'banner', FOOTER:'contentinfo', FORM:'form', DIALOG:'dialog', SECTION:'region', ARTICLE:'article', H1:'heading', H2:'heading', H3:'heading', H4:'heading', H5:'heading', H6:'heading', LABEL:'label', P:'paragraph', UL:'list', OL:'list', LI:'listitem' };
    return map[tag] || '';
  }
  function snapshotName(el, role){
    if(!el) return '';
    if(role === 'textbox' || role === 'combobox') return txt(el.getAttribute('placeholder') || labelFor(el) || el.value || '', 80);
    if(role === 'button' || role === 'link') return txt(el.innerText || el.textContent || el.value || el.getAttribute('title') || '', 80);
    if(role === 'img') return txt(el.getAttribute('alt') || el.getAttribute('aria-label') || el.getAttribute('title') || '', 80);
    if(role === 'form') return txt(headingText(el.closest('.card,.panel,[class*="card"],[class*="panel"]') || el) || el.getAttribute('aria-label') || '', 80);
    return txt(el.innerText || el.textContent || el.getAttribute('aria-label') || el.getAttribute('title') || '', 80);
  }
  function isInteractiveRole(role){ return /^(button|link|textbox|combobox|checkbox|radio|slider)$/.test(role); }
  function visibleRoot(selector){
    if(!selector) return null;
    var nodes;
    try { nodes = document.querySelectorAll(selector); } catch (e) { return null; }
    for(var i=0;i<nodes.length;i++){
      if(vis(nodes[i])) return nodes[i];
    }
    return null;
  }
  function chooseSnapshotRoot(){
    var explicit = visibleRoot(txt(cfg.snapshot_selector || '', 160));
    if(explicit) return explicit;
    var selectors = ['main', '[role="main"]', 'dialog,[role="dialog"]', 'form', '.modal,[class*="dialog"]', '.card,[class*="card"]', 'body'];
    for(var i=0;i<selectors.length;i++){
      var node = visibleRoot(selectors[i]);
      if(node) return node;
    }
    return document.body;
  }
  var snapshotNodeLimit = Math.max(10, Number(cfg.structured_nodes) || 60);
  var snapshotDepthLimit = Math.max(2, Number(cfg.structured_depth) || 6);
  var snapshotRefLimit = Math.max(4, Number(cfg.structured_refs) || 10);
  var snapshotLines = [];
  var interactionRefs = [];
  var snapshotCount = 0;
  var refSeq = 1;
  function addInteractionRef(el, role, name){
    if(!isInteractiveRole(role) || interactionRefs.length >= snapshotRefLimit) return '';
    var selector = sel(el);
    if(!selector) return '';
    for(var i=0;i<interactionRefs.length;i++){
      if(interactionRefs[i].selector === selector) return interactionRefs[i].ref;
    }
    var ref = 'r' + refSeq++;
    interactionRefs.push({ ref: ref, selector: selector, role: role, name: txt(name, 80) });
    return ref;
  }
  function snapshotWalk(node, depth){
    if(!node || node.nodeType !== 1 || depth > snapshotDepthLimit || snapshotCount >= snapshotNodeLimit) return;
    if(node !== document.body && !vis(node)) return;
    var role = treeRole(node);
    var include = depth === 0 || role !== '';
    var nextDepth = depth;
    if(include){
      if(!role) role = 'generic';
      var name = snapshotName(node, role);
      var indent = new Array(depth + 1).join('  ');
      var line = indent + '- ' + role;
      if(name) line += ' "' + name.replace(/"/g, '\\"') + '"';
      if(role === 'heading' && /^H[1-6]$/.test(node.tagName)) line += ' [level=' + node.tagName.slice(1) + ']';
      var ref = addInteractionRef(node, role, name);
      if(ref) line += ' [ref=' + ref + ']';
      if(node === document.activeElement) line += ' [active]';
      snapshotLines.push(line);
      snapshotCount++;
      nextDepth = depth + 1;
    }
    var children = node.children || [];
    for(var i=0;i<children.length && snapshotCount < snapshotNodeLimit;i++) snapshotWalk(children[i], nextDepth);
  }
  var alerts = texts('[role="alert"],.alert,.error,.errors,.message,.notice,.toast,.invalid-feedback,.form-error,.help.is-danger', 6);
  var validation = [];
  var fields = document.querySelectorAll('input,textarea,select');
  for(var j=0;j<fields.length;j++){
    var f = fields[j];
    if(typeof f.checkValidity === 'function' && !f.checkValidity()){
      var name = txt(f.name || f.id || f.placeholder || f.type || f.tagName.toLowerCase(), 40);
      var msg = txt(f.validationMessage || '', 120);
      if(msg) validation.push(name + ': ' + msg);
    }
  }
  var keyTexts = texts('h1,h2,h3,button,[role="button"],label,.alert,.error,.toast,.notice,.message,[aria-live]', 10);
  var watched = [];
  var watch = Array.isArray(cfg.watch_selectors) ? cfg.watch_selectors : [];
  for(var k=0;k<watch.length;k++){
    var q = watch[k];
    try {
      var node = document.querySelector(q);
      watched.push({ selector: q, exists: !!node, visible: !!node && vis(node), text: node ? safeValue(node) : '' });
    } catch (err) {
      watched.push({ selector: q, exists: false, visible: false, error: txt(err.message || String(err), 120) });
    }
  }
  var forms = [];
  var formNodes = document.querySelectorAll('form');
  for(var n=0;n<formNodes.length;n++){
    var formNode = formNodes[n];
    if(!vis(formNode)) continue;
    forms.push(summarizeForm(formNode));
    if(forms.length >= 4) break;
  }
  if(forms.length === 0){
    var synthetic = syntheticForm();
    if(synthetic) forms.push(synthetic);
  }
  var snapshotRoot = chooseSnapshotRoot();
  var layoutHints = [];
  if(forms.length === 1) layoutHints.push('single_form_page');
  if(forms.length > 1) layoutHints.push('multi_form_page');
  if(snapshotRoot && snapshotRoot.tagName){
    var rootTag = snapshotRoot.tagName.toLowerCase();
    if(rootTag === 'main') layoutHints.push('main_root');
    if(rootTag === 'dialog') layoutHints.push('dialog_root');
  }
  if(forms.length > 0 && forms[0].field_count > 0 && forms[0].field_count <= 4) layoutHints.push('compact_form');
  var mainHeading = texts('h1,h2,h3,legend,.card-title,.form-title,.panel-title', 4)[0] || '';
  snapshotWalk(snapshotRoot || document.body, 0);
  var structuredSnapshot = snapshotLines.join('\n');
  var active = document.activeElement ? sel(document.activeElement) : '';
  return { alerts: alerts, validation: validation, key_texts: keyTexts, watched: watched, page_frame: { main_heading: mainHeading, layout_hints: layoutHints, forms: forms }, structured_snapshot: structuredSnapshot, interaction_refs: interactionRefs, active_element: active };
}`
