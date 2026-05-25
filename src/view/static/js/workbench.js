/**
 * workbench.js — 工作台前端交互逻辑
 * 依赖全局变量 WB_CONFIG（由 workbench.html 模板注入），ES5 兼容，无第三方依赖
 */
(function() {
    'use strict';

    // -- CSRF 和 API 请求 --

    function getCSRF() {
        if (WB_CONFIG && WB_CONFIG.csrfToken) return WB_CONFIG.csrfToken;
        var m = document.cookie.match(/(?:^|;\s*)_csrf_token=([^;]*)/);
        return m ? m[1] : '';
    }

    function apiRequest(url, opts) {
        opts = opts || {};
        opts.headers = opts.headers || {};
        opts.headers['Content-Type'] = 'application/json';
        opts.headers['X-CSRF-Token'] = getCSRF();
        opts.headers['X-Requested-With'] = 'fetch';
        return fetch(url, opts).then(function(r) { return r.json(); });
    }

    // -- 辅助函数 --

    function $(id) { return document.getElementById(id); }

    var scrollTimer = null;
    function scrollToBottom() {
        if (scrollTimer) return;
        scrollTimer = setTimeout(function() {
            scrollTimer = null;
            if (wbMessages) wbMessages.scrollTop = wbMessages.scrollHeight;
        }, 50);
    }

    function escapeHtml(str) {
        var d = document.createElement('div');
        d.textContent = str;
        return d.innerHTML;
    }

    function formatTokens(n) {
        if (!n || n <= 0) return '';
        if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
        return '' + n;
    }

    function emptyHtml(text) {
        return '<div class="wb-empty">' +
            '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">' +
            '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>' +
            '</svg><span>' + text + '</span></div>';
    }

    function show(el) { if (el) el.classList.remove('wb-hidden'); }
    function hide(el) { if (el) el.classList.add('wb-hidden'); }

    // -- DOM 元素引用 --

    var wbSelWorkspace    = $('wb-sel-workspace');
    var wbBtnNewWs        = $('wb-btn-new-workspace');
    var wbToolbarWsInfo   = $('wb-toolbar-ws-info');
    var wbSidebar         = $('wb-sidebar');
    var wbSidebarOverlay  = $('wb-sidebar-overlay');
    var wbToggleSidebar   = $('wb-toggle-sidebar');
    var wbSessionList     = $('wb-session-list');
    var wbBtnNewSession   = $('wb-btn-new-session');
    var wbMessages        = $('wb-messages');
    var wbInput           = $('wb-input');
    var wbBtnSend         = $('wb-btn-send');
    var wbSelModel        = $('wb-sel-model');
    var wbSelAgent        = $('wb-sel-agent');

    // 模态框元素
    var modalOverlay  = $('wb-modal-create-ws');
    var modalClose    = $('wb-modal-close');
    var modalCancel   = $('wb-modal-cancel');
    var modalSubmit   = $('wb-modal-submit');
    var wsName        = $('ws-name');
    var wsDesc        = $('ws-desc');
    var wsProject     = $('ws-project');

    // 任务面板元素
    var wbToggleTasks    = $('wb-toggle-tasks');
    var wbTaskPanel      = $('wb-task-panel');
    var wbTaskClose      = $('wb-task-close');
    var wbTaskStats      = $('wb-task-stats');
    var wbTaskList       = $('wb-task-list');
    var wbTaskExpandAll  = $('wb-task-expand-all');
    var wbTaskCollapseAll = $('wb-task-collapse-all');

    // 文档面板元素
    var wbToggleDocs  = $('wb-toggle-docs');
    var wbDocsPanel   = $('wb-docs-panel');
    var wbDocsClose   = $('wb-docs-close');
    var wbDocsList    = $('wb-docs-list');

    // 项目文档面板元素
    var wbToggleProjDocs = $('wb-toggle-proj-docs');
    var wbProjDocsPanel  = $('wb-proj-docs-panel');
    var wbProjDocsClose  = $('wb-proj-docs-close');
    var wbProjDocsList   = $('wb-proj-docs-list');

    // 文档查看模态框元素
    var docModalOverlay = $('wb-modal-view-doc');
    var docModalClose   = $('wb-doc-modal-close');
    var docModalTitle   = $('wb-doc-modal-title');
    var docContent      = $('wb-doc-content');
    var docEditBtn      = $('wb-doc-edit-btn');
    var docSaveBtn      = $('wb-doc-save-btn');
    var docCancelEditBtn = $('wb-doc-cancel-edit-btn');
    var docEditor       = $('wb-doc-editor');

    // 项目文件面板元素
    var wbToggleFiles  = $('wb-toggle-files');
    var wbFilesPanel   = $('wb-files-panel');
    var wbFilesClose   = $('wb-files-close');
    var wbFilesList    = $('wb-files-list');

    // -- 状态变量 --

    var currentConvId  = null;
    var sending        = false;
    var lastSeq        = WB_CONFIG.lastSeq || 0; // 最后收到的事件序号（重连时回传）
    var subConvStatus  = {};          // {convId: 'running'|'done'|'error'}
    var reconnectTimer = null;
    var activeTask     = false;       // 是否有任务正在进行（用于断线重连判断）
    var taskPanelVisible   = false;   // 任务面板是否可见
    var docsPanelVisible   = false;   // 文档面板是否可见
    var projDocsPanelVisible = false; // 项目文档面板是否可见
    var filesPanelVisible  = false;   // 项目文件面板是否可见
    var editingDocInfo     = null;    // 当前编辑的文档信息 {type, path, rawContent}
    var pendingTaskRefresh = false;   // 是否有待刷新的任务变更
    var taskRefreshTimer   = null;    // 任务刷新防抖定时器
    var taskExpandedState  = {};      // 任务折叠状态 {taskId: true|false}
    var phaseExpandedState = {};      // 阶段折叠状态 {phase: true|false}

    function setSendButtonState(mode) {
        if (!wbBtnSend) return;
        if (mode === 'sending') {
            wbBtnSend.textContent = '停止';
            wbBtnSend.disabled = false;
            wbBtnSend.classList.add('wb-stop-btn');
        } else {
            wbBtnSend.textContent = '发送';
            wbBtnSend.disabled = false;
            wbBtnSend.classList.remove('wb-stop-btn');
        }
    }

    function stopConversation() {
        if (!currentConvId || !WB_CONFIG.workspaceId) return;
        wbBtnSend.disabled = true;
        wbBtnSend.textContent = '停止中...';
        apiRequest('/admin/workbench/conversations/' + currentConvId + '/stop?workspace_id=' + WB_CONFIG.workspaceId, {
            method: 'POST',
            body: JSON.stringify({})
        }).then(function(resp) {
            if (!resp.data || !resp.data.stopped) {
                setSendButtonState(activeTask ? 'sending' : 'idle');
            }
        }).catch(function() {
            setSendButtonState(activeTask ? 'sending' : 'idle');
        });
    }

    // -- 工作区选择 --

    if (wbSelWorkspace) {
        wbSelWorkspace.addEventListener('change', function() {
            var id = this.value;
            window.location.href = id ? '/admin/workbench?workspace_id=' + id : '/admin/workbench';
        });
    }

    // 顶栏显示当前工作区名称
    function updateToolbarInfo() {
        if (!wbToolbarWsInfo || !wbSelWorkspace) return;
        var opt = wbSelWorkspace.options[wbSelWorkspace.selectedIndex];
        wbToolbarWsInfo.textContent = (opt && opt.value) ? opt.text : '';
    }

    // -- 创建工作区模态框 --

    function openModal() {
        if (!modalOverlay) return;
        if (wsName)    wsName.value    = '';
        if (wsDesc)    wsDesc.value    = '';
        if (wsProject) wsProject.value = '';
        modalOverlay.classList.add('active');
    }

    function closeModal() {
        if (modalOverlay) modalOverlay.classList.remove('active');
    }

    if (wbBtnNewWs)   wbBtnNewWs.addEventListener('click', openModal);
    if (modalClose)   modalClose.addEventListener('click', closeModal);
    if (modalCancel)  modalCancel.addEventListener('click', closeModal);

    if (modalOverlay) {
        modalOverlay.addEventListener('click', function(e) {
            if (e.target === modalOverlay) closeModal();
        });
    }

    if (modalSubmit) {
        modalSubmit.addEventListener('click', function() {
            var name = wsName ? wsName.value.trim() : '';
            var desc = wsDesc ? wsDesc.value.trim() : '';

            if (!name) { alert('请输入工作区名称'); if (wsName) wsName.focus(); return; }

            var body = { name: name, description: desc };
            if (wsProject && wsProject.value) {
                body.project_id = parseInt(wsProject.value, 10) || 0;
            }

            modalSubmit.disabled = true;
            modalSubmit.textContent = '创建中...';

            apiRequest('/admin/workbench/workspaces', {
                method: 'POST',
                body: JSON.stringify(body)
            }).then(function(resp) {
                modalSubmit.disabled = false;
                modalSubmit.textContent = '创建';
                if (resp.data && resp.data.id) {
                    closeModal();
                    window.location.href = '/admin/workbench?workspace_id=' + resp.data.id;
                } else {
                    alert(resp.message || '创建失败');
                }
            }).catch(function() {
                modalSubmit.disabled = false;
                modalSubmit.textContent = '创建';
                alert('网络错误，请重试');
            });
        });
    }

    // -- 移动端侧边栏切换 --

    if (wbToggleSidebar) {
        wbToggleSidebar.addEventListener('click', function() {
            if (wbSidebar) wbSidebar.classList.toggle('wb-sidebar-open');
            if (wbSidebarOverlay) wbSidebarOverlay.classList.toggle('active');
        });
    }
    if (wbSidebarOverlay) {
        wbSidebarOverlay.addEventListener('click', function() {
            if (wbSidebar) wbSidebar.classList.remove('wb-sidebar-open');
            wbSidebarOverlay.classList.remove('active');
        });
    }

    // -- 任务面板切换 --

    function showTaskPanel() {
        if (docsPanelVisible) hideDocsPanel();
        if (projDocsPanelVisible) hideProjDocsPanel();
        if (filesPanelVisible) hideFilesPanel();
        taskPanelVisible = true;
        if (wbTaskPanel) wbTaskPanel.classList.remove('wb-hidden');
        if (wbToggleTasks) wbToggleTasks.classList.add('active');
        try { localStorage.setItem('wb_right_panel', 'tasks'); } catch(e) {}
        loadTasks();
    }

    function hideTaskPanel() {
        taskPanelVisible = false;
        if (wbTaskPanel) wbTaskPanel.classList.add('wb-hidden');
        if (wbToggleTasks) wbToggleTasks.classList.remove('active');
        try { localStorage.setItem('wb_right_panel', ''); } catch(e) {}
    }

    function toggleTaskPanel() {
        if (taskPanelVisible) hideTaskPanel();
        else showTaskPanel();
    }

    if (wbToggleTasks) wbToggleTasks.addEventListener('click', toggleTaskPanel);
    if (wbTaskClose) wbTaskClose.addEventListener('click', hideTaskPanel);

    if (wbTaskExpandAll) wbTaskExpandAll.addEventListener('click', function() {
        var phases = wbTaskList ? wbTaskList.querySelectorAll('.wb-task-phase') : [];
        for (var i = 0; i < phases.length; i++) {
            phases[i].classList.add('wb-expanded');
            var phaseKey = phases[i].querySelector('.wb-task-phase-badge');
            if (phaseKey) phaseExpandedState[phaseKey.textContent.replace('P', '')] = true;
        }
        var nodes = wbTaskList ? wbTaskList.querySelectorAll('.wb-task-node-parent') : [];
        for (var j = 0; j < nodes.length; j++) {
            nodes[j].classList.add('wb-expanded');
            var idEl = nodes[j].querySelector('.wb-task-id');
            if (idEl) taskExpandedState[idEl.textContent.replace('#', '')] = true;
        }
    });

    if (wbTaskCollapseAll) wbTaskCollapseAll.addEventListener('click', function() {
        var phases = wbTaskList ? wbTaskList.querySelectorAll('.wb-task-phase') : [];
        for (var i = 0; i < phases.length; i++) {
            phases[i].classList.remove('wb-expanded');
            var phaseKey = phases[i].querySelector('.wb-task-phase-badge');
            if (phaseKey) phaseExpandedState[phaseKey.textContent.replace('P', '')] = false;
        }
        var nodes = wbTaskList ? wbTaskList.querySelectorAll('.wb-task-node-parent') : [];
        for (var j = 0; j < nodes.length; j++) {
            nodes[j].classList.remove('wb-expanded');
            var idEl = nodes[j].querySelector('.wb-task-id');
            if (idEl) taskExpandedState[idEl.textContent.replace('#', '')] = false;
        }
    });

    // -- 文档面板切换 --

    function showDocsPanel() {
        if (taskPanelVisible) hideTaskPanel();
        if (projDocsPanelVisible) hideProjDocsPanel();
        if (filesPanelVisible) hideFilesPanel();
        docsPanelVisible = true;
        if (wbDocsPanel) wbDocsPanel.classList.remove('wb-hidden');
        if (wbToggleDocs) wbToggleDocs.classList.add('active');
        try { localStorage.setItem('wb_right_panel', 'docs'); } catch(e) {}
        loadDocs();
    }

    function hideDocsPanel() {
        docsPanelVisible = false;
        if (wbDocsPanel) wbDocsPanel.classList.add('wb-hidden');
        if (wbToggleDocs) wbToggleDocs.classList.remove('active');
        try { localStorage.setItem('wb_right_panel', ''); } catch(e) {}
    }

    function toggleDocsPanel() {
        if (docsPanelVisible) hideDocsPanel();
        else showDocsPanel();
    }

    if (wbToggleDocs) wbToggleDocs.addEventListener('click', toggleDocsPanel);
    if (wbDocsClose) wbDocsClose.addEventListener('click', hideDocsPanel);

    // -- 项目文档面板切换 --

    function showProjDocsPanel() {
        if (docsPanelVisible) hideDocsPanel();
        if (taskPanelVisible) hideTaskPanel();
        if (filesPanelVisible) hideFilesPanel();
        projDocsPanelVisible = true;
        if (wbProjDocsPanel) wbProjDocsPanel.classList.remove('wb-hidden');
        if (wbToggleProjDocs) wbToggleProjDocs.classList.add('active');
        try { localStorage.setItem('wb_right_panel', 'proj-docs'); } catch(e) {}
        loadProjDocs();
    }

    function hideProjDocsPanel() {
        projDocsPanelVisible = false;
        if (wbProjDocsPanel) wbProjDocsPanel.classList.add('wb-hidden');
        if (wbToggleProjDocs) wbToggleProjDocs.classList.remove('active');
        try { localStorage.setItem('wb_right_panel', ''); } catch(e) {}
    }

    function toggleProjDocsPanel() {
        if (projDocsPanelVisible) hideProjDocsPanel();
        else showProjDocsPanel();
    }

    if (wbToggleProjDocs) wbToggleProjDocs.addEventListener('click', toggleProjDocsPanel);
    if (wbProjDocsClose) wbProjDocsClose.addEventListener('click', hideProjDocsPanel);

    // -- Markdown 渲染 --

    function renderMarkdown(content, container) {
        if (typeof marked !== 'undefined') {
            marked.setOptions({ breaks: true, gfm: true });
            container.innerHTML = marked.parse(content);
            container.className = 'wb-doc-content wb-md-content';
        } else {
            container.textContent = content;
            container.className = 'wb-doc-content';
        }
    }

    // -- 文档加载与渲染 --

    function loadDocs() {
        if (!WB_CONFIG.workspaceId) return;
        apiRequest('/admin/workbench/docs?workspace_id=' + WB_CONFIG.workspaceId)
            .then(function(resp) {
                var data = resp.data || {};
                var docs = Array.isArray(data) ? data : (data.docs || []);
                var totalTokens = data.total_tokens || 0;
                renderDocsList(docs, totalTokens);
            });
    }

    function renderDocsList(docs, totalTokens) {
        if (!wbDocsList) return;
        var docsTokensEl = $('wb-docs-tokens');
        if (docsTokensEl) {
            docsTokensEl.textContent = totalTokens > 0 ? '~' + formatTokens(totalTokens) + ' tokens' : '';
        }
        if (!docs || !docs.length) {
            wbDocsList.innerHTML = '<div class="wb-empty" style="padding:2rem 0;"><span>暂无文档</span></div>';
            return;
        }

        var scopes = {};
        var scopeOrder = [];
        for (var i = 0; i < docs.length; i++) {
            var d = docs[i];
            var s = d.scope || 'other';
            if (!scopes[s]) {
                scopes[s] = [];
                scopeOrder.push(s);
            }
            scopes[s].push(d);
        }

        var scopeLabels = { specs: '规格文档', tasks: '任务文档' };
        wbDocsList.innerHTML = '';
        for (var si = 0; si < scopeOrder.length; si++) {
            var scope = scopeOrder[si];
            var files = scopes[scope];
            var scopeDiv = document.createElement('div');
            scopeDiv.className = 'wb-docs-scope';
            scopeDiv.innerHTML = '<div class="wb-docs-scope-header">' +
                '<span class="wb-docs-scope-badge">' + escapeHtml(scope) + '</span>' +
                '<span>' + escapeHtml(scopeLabels[scope] || scope) + ' (' + files.length + ')</span></div>';

            for (var fi = 0; fi < files.length; fi++) {
                (function(file) {
                    var item = document.createElement('div');
                    item.className = 'wb-doc-item';
                    var tokenText = file.tokens ? ' · ~' + formatTokens(file.tokens) + ' tokens' : '';
                    item.innerHTML =
                        '<span class="wb-doc-icon"><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></span>' +
                        '<div class="wb-doc-info">' +
                        '<div class="wb-doc-name" title="' + escapeHtml(file.path) + '">' + escapeHtml(file.name) + '</div>' +
                        '<div class="wb-doc-meta">' + escapeHtml(file.size_text) + ' · ' + escapeHtml(file.mod_time) + tokenText + '</div></div>';
                    var btn = document.createElement('button');
                    btn.className = 'wb-doc-view-btn';
                    btn.textContent = '查看';
                    btn.addEventListener('click', function(e) {
                        e.stopPropagation();
                        viewDoc(file.path);
                    });
                    item.appendChild(btn);
                    scopeDiv.appendChild(item);
                })(files[fi]);
            }

            wbDocsList.appendChild(scopeDiv);
        }
    }

    // -- 文档查看模态框 --

    function viewDoc(path) {
        if (!WB_CONFIG.workspaceId || !path) return;
        editingDocInfo = { type: 'docs', path: path };
        if (docModalTitle) docModalTitle.textContent = path;
        if (docContent) { docContent.textContent = '加载中...'; docContent.classList.remove('wb-hidden'); }
        if (docEditor) docEditor.classList.add('wb-hidden');
        if (docEditBtn) docEditBtn.classList.remove('wb-hidden');
        if (docSaveBtn) docSaveBtn.classList.add('wb-hidden');
        if (docCancelEditBtn) docCancelEditBtn.classList.add('wb-hidden');
        if (docModalOverlay) docModalOverlay.classList.add('active');

        apiRequest('/admin/workbench/docs/content?workspace_id=' + WB_CONFIG.workspaceId + '&path=' + encodeURIComponent(path))
            .then(function(resp) {
                if (resp.data && resp.data.content !== undefined) {
                    editingDocInfo.rawContent = resp.data.content;
                    if (docContent) renderMarkdown(resp.data.content, docContent);
                } else {
                    if (docContent) docContent.textContent = resp.message || '读取失败';
                }
            })
            .catch(function() {
                if (docContent) docContent.textContent = '网络错误，读取失败';
            });
    }

    function closeDocModal() {
        if (docModalOverlay) docModalOverlay.classList.remove('active');
        exitEditMode();
        editingDocInfo = null;
    }

    if (docModalClose) docModalClose.addEventListener('click', closeDocModal);
    if (docModalOverlay) {
        docModalOverlay.addEventListener('click', function(e) {
            if (e.target === docModalOverlay) closeDocModal();
        });
    }

    // -- 项目文档加载与渲染 --

    function loadProjDocs() {
        if (!WB_CONFIG.workspaceId) return;
        apiRequest('/admin/workbench/project-docs?workspace_id=' + WB_CONFIG.workspaceId)
            .then(function(resp) {
                var data = resp.data || {};
                var docs = Array.isArray(data) ? data : (data.docs || []);
                var totalTokens = data.total_tokens || 0;
                renderProjDocsList(docs, totalTokens);
            });
    }

    function renderProjDocsList(docs, totalTokens) {
        if (!wbProjDocsList) return;
        var tokensEl = $('wb-proj-docs-tokens');
        if (tokensEl) {
            tokensEl.textContent = totalTokens > 0 ? '~' + formatTokens(totalTokens) + ' tokens' : '';
        }
        if (!docs || !docs.length) {
            wbProjDocsList.innerHTML = '<div class="wb-empty" style="padding:2rem 0;"><span>暂无文档</span></div>';
            return;
        }

        var scopes = {};
        var scopeOrder = [];
        for (var i = 0; i < docs.length; i++) {
            var d = docs[i];
            var s = d.scope || 'other';
            if (!scopes[s]) {
                scopes[s] = [];
                scopeOrder.push(s);
            }
            scopes[s].push(d);
        }

        var scopeLabels = { root: '项目根目录', docs: '文档目录' };
        wbProjDocsList.innerHTML = '';
        for (var si = 0; si < scopeOrder.length; si++) {
            var scope = scopeOrder[si];
            var files = scopes[scope];
            var scopeDiv = document.createElement('div');
            scopeDiv.className = 'wb-docs-scope';
            scopeDiv.innerHTML = '<div class="wb-docs-scope-header">' +
                '<span class="wb-docs-scope-badge">' + escapeHtml(scope) + '</span>' +
                '<span>' + escapeHtml(scopeLabels[scope] || scope) + ' (' + files.length + ')</span></div>';

            for (var fi = 0; fi < files.length; fi++) {
                (function(file) {
                    var item = document.createElement('div');
                    item.className = 'wb-doc-item';
                    var tokenText = file.tokens ? ' · ~' + formatTokens(file.tokens) + ' tokens' : '';
                    item.innerHTML =
                        '<span class="wb-doc-icon"><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></span>' +
                        '<div class="wb-doc-info">' +
                        '<div class="wb-doc-name" title="' + escapeHtml(file.path) + '">' + escapeHtml(file.name) + '</div>' +
                        '<div class="wb-doc-meta">' + escapeHtml(file.size_text) + ' · ' + escapeHtml(file.mod_time) + tokenText + '</div></div>';
                    var btn = document.createElement('button');
                    btn.className = 'wb-doc-view-btn';
                    btn.textContent = '查看';
                    btn.addEventListener('click', function(e) {
                        e.stopPropagation();
                        viewProjDoc(file.path);
                    });
                    item.appendChild(btn);
                    scopeDiv.appendChild(item);
                })(files[fi]);
            }

            wbProjDocsList.appendChild(scopeDiv);
        }
    }

    function viewProjDoc(path) {
        if (!WB_CONFIG.workspaceId || !path) return;
        editingDocInfo = { type: 'proj-docs', path: path };
        if (docModalTitle) docModalTitle.textContent = path;
        if (docContent) { docContent.textContent = '加载中...'; docContent.classList.remove('wb-hidden'); }
        if (docEditor) docEditor.classList.add('wb-hidden');
        if (docEditBtn) docEditBtn.classList.remove('wb-hidden');
        if (docSaveBtn) docSaveBtn.classList.add('wb-hidden');
        if (docCancelEditBtn) docCancelEditBtn.classList.add('wb-hidden');
        if (docModalOverlay) docModalOverlay.classList.add('active');

        apiRequest('/admin/workbench/project-docs/content?workspace_id=' + WB_CONFIG.workspaceId + '&path=' + encodeURIComponent(path))
            .then(function(resp) {
                if (resp.data && resp.data.content !== undefined) {
                    editingDocInfo.rawContent = resp.data.content;
                    if (docContent) renderMarkdown(resp.data.content, docContent);
                } else {
                    if (docContent) docContent.textContent = resp.message || '读取失败';
                }
            })
            .catch(function() {
                if (docContent) docContent.textContent = '网络错误，读取失败';
            });
    }

    // -- 项目文件面板切换 --

    function showFilesPanel() {
        if (docsPanelVisible) hideDocsPanel();
        if (projDocsPanelVisible) hideProjDocsPanel();
        if (taskPanelVisible) hideTaskPanel();
        filesPanelVisible = true;
        if (wbFilesPanel) wbFilesPanel.classList.remove('wb-hidden');
        if (wbToggleFiles) wbToggleFiles.classList.add('active');
        try { localStorage.setItem('wb_right_panel', 'files'); } catch(e) {}
        loadProjectFiles('');
    }

    function hideFilesPanel() {
        filesPanelVisible = false;
        if (wbFilesPanel) wbFilesPanel.classList.add('wb-hidden');
        if (wbToggleFiles) wbToggleFiles.classList.remove('active');
        try { localStorage.setItem('wb_right_panel', ''); } catch(e) {}
    }

    function toggleFilesPanel() {
        if (filesPanelVisible) hideFilesPanel();
        else showFilesPanel();
    }

    if (wbToggleFiles) wbToggleFiles.addEventListener('click', toggleFilesPanel);
    if (wbFilesClose) wbFilesClose.addEventListener('click', hideFilesPanel);

    // -- 项目文件树加载与渲染 --

    function loadProjectFiles(path, container) {
        if (!WB_CONFIG.workspaceId) return;
        var url = '/admin/workbench/project-files?workspace_id=' + WB_CONFIG.workspaceId;
        if (path) url += '&path=' + encodeURIComponent(path);
        apiRequest(url).then(function(resp) {
            if (resp.success || resp.data) {
                var files = (resp.data && resp.data.files) || [];
                renderFileTree(files, container || wbFilesList);
            }
        });
    }

    function renderFileTree(files, container) {
        if (!container) return;
        if (!files || !files.length) {
            if (container === wbFilesList) {
                container.innerHTML = '<div class="wb-empty" style="padding:2rem 0;"><span>暂无文件</span></div>';
            }
            return;
        }
        if (container === wbFilesList) container.innerHTML = '';
        else container.innerHTML = '';

        for (var i = 0; i < files.length; i++) {
            (function(file) {
                var node = document.createElement('div');
                if (file.is_dir) {
                    node.innerHTML =
                        '<div class="wb-file-item">' +
                        '<svg class="wb-file-toggle" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 18l6-6-6-6"/></svg>' +
                        '<svg class="wb-file-icon wb-file-icon-dir" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>' +
                        '<span class="wb-file-name">' + escapeHtml(file.name) + '</span>' +
                        '<span class="wb-file-size">' + (file.children_count || '') + '</span>' +
                        '</div>' +
                        '<div class="wb-file-children"></div>';
                    var itemEl = node.querySelector('.wb-file-item');
                    var childrenEl = node.querySelector('.wb-file-children');
                    var toggleEl = node.querySelector('.wb-file-toggle');
                    var loaded = false;
                    itemEl.addEventListener('click', function() {
                        if (childrenEl.classList.contains('wb-expanded')) {
                            childrenEl.classList.remove('wb-expanded');
                            toggleEl.classList.remove('wb-expanded');
                        } else {
                            childrenEl.classList.add('wb-expanded');
                            toggleEl.classList.add('wb-expanded');
                            if (!loaded) {
                                loaded = true;
                                loadProjectFiles(file.path, childrenEl);
                            }
                        }
                    });
                } else {
                    node.innerHTML =
                        '<div class="wb-file-item">' +
                        '<svg class="wb-file-toggle" viewBox="0 0 24 24" style="visibility:hidden"><path/></svg>' +
                        '<svg class="wb-file-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>' +
                        '<span class="wb-file-name">' + escapeHtml(file.name) + '</span>' +
                        '<span class="wb-file-size">' + escapeHtml(file.size_text || '') + '</span>' +
                        '</div>';
                    var fileItemEl = node.querySelector('.wb-file-item');
                    fileItemEl.addEventListener('click', function() {
                        viewProjectFile(file.path, file.name);
                    });
                }
                container.appendChild(node);
            })(files[i]);
        }
    }

    function viewProjectFile(path, name) {
        if (!WB_CONFIG.workspaceId || !path) return;
        editingDocInfo = { type: 'proj-files', path: path };
        if (docModalTitle) docModalTitle.textContent = name || path;
        if (docContent) { docContent.textContent = '加载中...'; docContent.classList.remove('wb-hidden'); }
        if (docEditor) docEditor.classList.add('wb-hidden');
        if (docEditBtn) docEditBtn.classList.remove('wb-hidden');
        if (docSaveBtn) docSaveBtn.classList.add('wb-hidden');
        if (docCancelEditBtn) docCancelEditBtn.classList.add('wb-hidden');
        if (docModalOverlay) docModalOverlay.classList.add('active');

        apiRequest('/admin/workbench/project-files/content?workspace_id=' + WB_CONFIG.workspaceId + '&path=' + encodeURIComponent(path))
            .then(function(resp) {
                if (resp.data && resp.data.content !== undefined) {
                    editingDocInfo.rawContent = resp.data.content;
                    if (path.match(/\.md$/i)) {
                        renderMarkdown(resp.data.content, docContent);
                    } else {
                        if (docContent) { docContent.textContent = resp.data.content; docContent.className = 'wb-doc-content'; }
                    }
                } else {
                    if (docContent) docContent.textContent = resp.message || '读取失败';
                }
            })
            .catch(function() {
                if (docContent) docContent.textContent = '网络错误，读取失败';
            });
    }

    // -- 文档编辑/保存逻辑 --

    function enterEditMode() {
        if (!editingDocInfo) return;
        if (docContent) docContent.classList.add('wb-hidden');
        if (docEditor) {
            docEditor.value = editingDocInfo.rawContent || '';
            docEditor.classList.remove('wb-hidden');
        }
        if (docEditBtn) docEditBtn.classList.add('wb-hidden');
        if (docSaveBtn) docSaveBtn.classList.remove('wb-hidden');
        if (docCancelEditBtn) docCancelEditBtn.classList.remove('wb-hidden');
    }

    function exitEditMode() {
        if (docEditor) docEditor.classList.add('wb-hidden');
        if (docContent) docContent.classList.remove('wb-hidden');
        if (docEditBtn) docEditBtn.classList.remove('wb-hidden');
        if (docSaveBtn) docSaveBtn.classList.add('wb-hidden');
        if (docCancelEditBtn) docCancelEditBtn.classList.add('wb-hidden');
    }

    function saveDocContent() {
        if (!editingDocInfo || !WB_CONFIG.workspaceId) return;
        if (!confirm('确认保存修改？')) return;

        var urlMap = {
            'docs': '/admin/workbench/docs/save',
            'proj-docs': '/admin/workbench/project-docs/save',
            'proj-files': '/admin/workbench/project-files/save'
        };
        var url = urlMap[editingDocInfo.type];
        if (!url) return;

        var body = JSON.stringify({
            workspace_id: parseInt(WB_CONFIG.workspaceId),
            path: editingDocInfo.path,
            content: docEditor ? docEditor.value : ''
        });

        apiRequest(url, { method: 'POST', headers: {'Content-Type': 'application/json'}, body: body })
            .then(function(resp) {
                if (resp.success) {
                    editingDocInfo.rawContent = docEditor ? docEditor.value : '';
                    exitEditMode();
                    if (editingDocInfo.path.match(/\.md$/i)) {
                        renderMarkdown(editingDocInfo.rawContent, docContent);
                    } else {
                        if (docContent) { docContent.textContent = editingDocInfo.rawContent; docContent.className = 'wb-doc-content'; }
                    }
                } else {
                    alert(resp.message || '保存失败');
                }
            })
            .catch(function() {
                alert('网络错误，保存失败');
            });
    }

    if (docEditBtn) docEditBtn.addEventListener('click', enterEditMode);
    if (docSaveBtn) docSaveBtn.addEventListener('click', saveDocContent);
    if (docCancelEditBtn) docCancelEditBtn.addEventListener('click', exitEditMode);

    // -- 拖拽调整面板宽度 --

    (function initPanelResize() {
        var handles = document.querySelectorAll('.wb-resize-handle');
        var minWidth = 200, maxWidth = 600;

        for (var i = 0; i < handles.length; i++) {
            handles[i].addEventListener('mousedown', startResize);
        }

        function startResize(e) {
            e.preventDefault();
            var panel = e.target.parentElement;
            var startX = e.clientX;
            var startWidth = panel.offsetWidth;
            e.target.classList.add('wb-resizing');
            document.body.style.cursor = 'col-resize';
            document.body.style.userSelect = 'none';

            function onMove(ev) {
                var delta = startX - ev.clientX;
                var newWidth = Math.min(maxWidth, Math.max(minWidth, startWidth + delta));
                panel.style.width = newWidth + 'px';
            }
            function onUp() {
                document.removeEventListener('mousemove', onMove);
                document.removeEventListener('mouseup', onUp);
                e.target.classList.remove('wb-resizing');
                document.body.style.cursor = '';
                document.body.style.userSelect = '';
                try { localStorage.setItem('wb_panel_width', panel.offsetWidth); } catch(ex) {}
            }
            document.addEventListener('mousemove', onMove);
            document.addEventListener('mouseup', onUp);
        }

        try {
            var saved = localStorage.getItem('wb_panel_width');
            if (saved) {
                var w = parseInt(saved, 10);
                if (w >= minWidth && w <= maxWidth) {
                    document.documentElement.style.setProperty('--wb-panel-width', w + 'px');
                }
            }
        } catch(ex) {}
    })();

    // -- 任务加载与渲染 --

    function loadTasks() {
        if (!WB_CONFIG.workspaceId) return;
        apiRequest('/admin/workbench/tasks?workspace_id=' + WB_CONFIG.workspaceId)
            .then(function(resp) {
                if (!resp.data) return;
                renderTaskStats(resp.data.stats || {}, resp.data.task_summary_tokens || 0);
                renderTaskTree(resp.data.tasks || []);
            });
    }

    function scheduleTaskRefresh() {
        if (taskRefreshTimer) clearTimeout(taskRefreshTimer);
        taskRefreshTimer = setTimeout(function() {
            taskRefreshTimer = null;
            loadTasks();
        }, 500);
    }

    function renderTaskStats(stats, taskTokens) {
        if (!wbTaskStats) return;
        var total = stats.total || 0;
        var running = stats.running || 0;
        var completed = stats.completed || 0;
        var failed = stats.failed || 0;
        var tokenHtml = taskTokens > 0
            ? '<span class="wb-task-stat wb-task-stat-tokens"><span class="wb-task-stat-val">~' + formatTokens(taskTokens) + '</span> tokens</span>'
            : '';
        wbTaskStats.innerHTML =
            '<span class="wb-task-stat"><span class="wb-task-stat-val">' + total + '</span> 全部</span>' +
            '<span class="wb-task-stat wb-task-stat-running"><span class="wb-task-stat-val">' + running + '</span> 进行</span>' +
            '<span class="wb-task-stat wb-task-stat-done"><span class="wb-task-stat-val">' + completed + '</span> 完成</span>' +
            '<span class="wb-task-stat wb-task-stat-failed"><span class="wb-task-stat-val">' + failed + '</span> 失败</span>' +
            tokenHtml;
    }

    function renderTaskTree(tasks) {
        if (!wbTaskList) return;
        if (!tasks || !tasks.length) {
            wbTaskList.innerHTML = '<div class="wb-empty" style="padding:2rem 0;"><span>暂无任务</span></div>';
            return;
        }

        var phases = {};
        var phaseLabels = {};
        var phaseOrder = [];
        for (var i = 0; i < tasks.length; i++) {
            var t = tasks[i];
            var p = t.phase || 1;
            if (!phases[p]) {
                phases[p] = [];
                phaseOrder.push(p);
            }
            phases[p].push(t);
            if (t.phase_label) phaseLabels[p] = t.phase_label;
        }

        wbTaskList.innerHTML = '';
        for (var pi = 0; pi < phaseOrder.length; pi++) {
            (function(phase) {
                var phaseTasks = phases[phase];
                var label = phaseLabels[phase] || '';
                var tree = buildTaskTree(phaseTasks);
                var phaseExpanded = isPhaseExpanded(phase);

                var phaseDiv = document.createElement('div');
                phaseDiv.className = 'wb-task-phase' + (phaseExpanded ? ' wb-expanded' : '');

                var headerDiv = document.createElement('div');
                headerDiv.className = 'wb-task-phase-header';
                headerDiv.innerHTML =
                    '<svg class="wb-task-phase-toggle" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
                        '<polyline points="9 18 15 12 9 6"></polyline>' +
                    '</svg>' +
                    '<span class="wb-task-phase-badge">P' + phase + '</span>' +
                    (label ? '<span>' + escapeHtml(label) + '</span>' : '');
                headerDiv.addEventListener('click', function() {
                    phaseExpandedState[phase] = !phaseDiv.classList.contains('wb-expanded');
                    phaseDiv.classList.toggle('wb-expanded');
                });
                phaseDiv.appendChild(headerDiv);

                var bodyDiv = document.createElement('div');
                bodyDiv.className = 'wb-task-phase-body';
                for (var k = 0; k < tree.roots.length; k++) {
                    bodyDiv.appendChild(createTaskNode(tree.roots[k], tree.childMap, 0));
                }
                phaseDiv.appendChild(bodyDiv);

                wbTaskList.appendChild(phaseDiv);
            })(phaseOrder[pi]);
        }
    }

    function isPhaseExpanded(phase) {
        if (phaseExpandedState.hasOwnProperty(phase)) return !!phaseExpandedState[phase];
        return false;
    }

    function buildTaskTree(tasks) {
        var roots = [];
        var childMap = {};
        var taskMap = {};

        for (var i = 0; i < tasks.length; i++) {
            taskMap[tasks[i].id] = tasks[i];
        }

        for (var j = 0; j < tasks.length; j++) {
            var task = tasks[j];
            var parentId = task.parent_id || 0;
            if (!parentId || !taskMap[parentId]) {
                roots.push(task);
            } else {
                if (!childMap[parentId]) childMap[parentId] = [];
                childMap[parentId].push(task);
            }
        }

        return { roots: roots, childMap: childMap };
    }

    function createTaskNode(task, childMap, depth) {
        var children = childMap[task.id] || [];
        var hasChildren = children.length > 0;
        var expanded = hasChildren ? isTaskExpanded(task, children, childMap) : false;

        var wrapper = document.createElement('div');
        wrapper.className = 'wb-task-node' + (hasChildren ? ' wb-task-node-parent' : '') + (expanded ? ' wb-expanded' : '');

        var item = document.createElement('div');
        item.className = 'wb-task-item' + (depth > 0 ? ' wb-task-subtask' : '');
        item.style.paddingLeft = (0.375 + depth * 1.125) + 'rem';

        var icon = taskStatusIcon(task.status);
        var titleClass = 'wb-task-title' + (task.status >= 2 ? ' wb-task-title-done' : '');
        var titleHtml = '<span class="' + titleClass + '" title="' + escapeHtml(task.title) + '">' + escapeHtml(task.title) + '</span>';
        var metaHtml = '<span class="wb-task-id">#' + task.id + '</span>';

        if (hasChildren) {
            item.className += ' wb-task-item-collapsible';
            item.innerHTML =
                '<button type="button" class="wb-task-toggle" aria-label="切换子任务" aria-expanded="' + (expanded ? 'true' : 'false') + '">' +
                    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
                        '<polyline points="9 18 15 12 9 6"></polyline>' +
                    '</svg>' +
                '</button>' +
                '<span class="wb-task-icon ' + icon.cls + '">' + icon.ch + '</span>' +
                '<div class="wb-task-main">' + titleHtml + '</div>' +
                metaHtml;

            item.addEventListener('click', function() {
                taskExpandedState[task.id] = !wrapper.classList.contains('wb-expanded');
                wrapper.classList.toggle('wb-expanded');
                var toggle = item.querySelector('.wb-task-toggle');
                if (toggle) toggle.setAttribute('aria-expanded', wrapper.classList.contains('wb-expanded') ? 'true' : 'false');
            });
        } else {
            item.innerHTML =
                '<span class="wb-task-icon ' + icon.cls + '">' + icon.ch + '</span>' +
                '<div class="wb-task-main">' + titleHtml + '</div>' +
                metaHtml;
        }

        wrapper.appendChild(item);

        if (hasChildren) {
            var childrenWrap = document.createElement('div');
            childrenWrap.className = 'wb-task-children';
            for (var i = 0; i < children.length; i++) {
                childrenWrap.appendChild(createTaskNode(children[i], childMap, depth + 1));
            }
            wrapper.appendChild(childrenWrap);
        }

        return wrapper;
    }

    function isTaskExpanded(task, children, childMap) {
        if (taskExpandedState.hasOwnProperty(task.id)) return !!taskExpandedState[task.id];
        return false;
    }

    function taskStatusIcon(status) {
        switch (status) {
            case 0: return { ch: '\u00B7', cls: 'wb-task-icon-pending' };
            case 1: return { ch: '\u25B6', cls: 'wb-task-icon-running' };
            case 2: return { ch: '\u2713', cls: 'wb-task-icon-done' };
            case 3: return { ch: '\u2717', cls: 'wb-task-icon-failed' };
            case 4: return { ch: '\u2013', cls: 'wb-task-icon-skipped' };
            default: return { ch: '?', cls: '' };
        }
    }

    // -- 会话列表管理 --

    function loadConversations() {
        if (!WB_CONFIG.workspaceId) return;
        apiRequest('/admin/workbench/conversations?workspace_id=' + WB_CONFIG.workspaceId + '&type=all')
            .then(function(resp) {
                var convs = (resp.data && resp.data.conversations) || resp.data || [];
                if (!Array.isArray(convs)) convs = [];
                renderConversationList(convs);
            });
    }

    function renderConversationList(convs) {
        if (convs.length === 0) {
            wbSessionList.innerHTML = emptyHtml('暂无会话，发消息自动创建').replace('class="wb-empty"', 'class="wb-empty" style="padding:2rem 0;"');
            return;
        }
        wbSessionList.innerHTML = '';
        for (var j = 0; j < convs.length; j++) {
            (function(conv) {
                var div = document.createElement('div');
                div.className = 'wb-session-item' + (conv.id === currentConvId ? ' active' : '');
                div.setAttribute('data-id', conv.id);
                var agentLabel = '';
                if (conv.agent_id && WB_CONFIG.agents) {
                    for (var k = 0; k < WB_CONFIG.agents.length; k++) {
                        if (WB_CONFIG.agents[k].id === conv.agent_id) {
                            agentLabel = (WB_CONFIG.agents[k].title && WB_CONFIG.agents[k].name)
                                ? WB_CONFIG.agents[k].title + '-' + WB_CONFIG.agents[k].name
                                : (WB_CONFIG.agents[k].title || WB_CONFIG.agents[k].name || '');
                            break;
                        }
                    }
                }
                var isMain = conv.conv_type === 'main' && !conv.parent_id;
                var titleDisplay = conv.title || '新对话';
                var badgeStatus = subConvStatus[conv.id] || (conv.status === 1 ? 'running' : (conv.status === 2 ? 'done' : (conv.status === 3 ? 'error' : '')));
                var badgeHtml = badgeStatus ? '<span class="wb-status-badge wb-status-' + badgeStatus + '"></span>' : '';
                var starHtml = isMain ? '<span class="wb-star">★</span>' : '';
                var tokenHtml = conv.total_tokens > 0
                    ? '<span class="wb-session-tokens">' + formatTokens(conv.total_tokens) + '</span>'
                    : '';
                div.innerHTML = badgeHtml + starHtml +
                    '<span class="wb-session-title">' + escapeHtml(titleDisplay) + '</span>' +
                    tokenHtml +
                    '<span class="wb-session-delete wb-conv-delete" data-id="' + conv.id + '">&times;</span>';
                div.addEventListener('click', function(e) {
                    if (e.target.classList.contains('wb-conv-delete')) {
                        deleteConversation(parseInt(e.target.getAttribute('data-id'), 10));
                        return;
                    }
                    selectConversation(conv.id);
                    if (wbSidebar) wbSidebar.classList.remove('wb-sidebar-open');
                    if (wbSidebarOverlay) wbSidebarOverlay.classList.remove('active');
                });
                wbSessionList.appendChild(div);
            })(convs[j]);
        }
    }

    function selectConversation(id) {
        window.location.href = '/admin/workbench?workspace_id=' + WB_CONFIG.workspaceId + '&conv_id=' + id;
    }

    // -- 新建会话 --

    if (wbBtnNewSession) {
        wbBtnNewSession.addEventListener('click', function() {
            if (!WB_CONFIG.workspaceId) { alert('请先选择工作区'); return; }
            var agentId = wbSelAgent && wbSelAgent.value ? parseInt(wbSelAgent.value, 10) : 0;
            var title = '新会话';
            apiRequest('/admin/workbench/conversations', {
                method: 'POST',
                body: JSON.stringify({
                    workspace_id: WB_CONFIG.workspaceId,
                    agent_id: agentId,
                    title: title,
                    conv_type: 'main'
                })
            }).then(function(resp) {
                if (resp.data && resp.data.id) {
                    window.location.href = '/admin/workbench?workspace_id=' + WB_CONFIG.workspaceId + '&conv_id=' + resp.data.id;
                } else { alert(resp.message || '创建失败'); }
            });
        });
    }

    // -- 删除会话 --

    function deleteConversation(id) {
        if (!confirm('确定要删除此会话吗？')) return;
        var url = '/admin/workbench/conversations/' + id;
        if (WB_CONFIG.workspaceId) url += '?workspace_id=' + WB_CONFIG.workspaceId;
        apiRequest(url, { method: 'DELETE' })
            .then(function() {
                if (currentConvId === id) {
                    // 删除当前会话，跳转回工作区页面
                    window.location.href = '/admin/workbench?workspace_id=' + WB_CONFIG.workspaceId;
                } else {
                    loadConversations();
                }
            });
    }

    // -- 消息加载和显示 --

    function updateConvTokens(tokens) {
        var el = $('wb-conv-tokens');
        if (!el) return;
        el.textContent = (tokens && tokens > 0) ? '~' + formatTokens(tokens) + ' tokens' : '';
    }

    function loadMessages(convId) {
        var url = '/admin/workbench/conversations/' + convId + '/messages';
        if (WB_CONFIG.workspaceId) url += '?workspace_id=' + WB_CONFIG.workspaceId;
        return apiRequest(url)
            .then(function(resp) {
                var payload = resp.data || {};
                var msgs, summary;
                if (Array.isArray(payload)) {
                    msgs = payload;
                    summary = '';
                } else {
                    msgs = payload.messages || [];
                    summary = payload.summary || '';
                }
                if (!Array.isArray(msgs)) msgs = [];
                renderMessages(msgs, summary);
                updateConvTokens(payload.total_tokens || 0);
            });
    }

    function renderMessages(msgs, summary) {
        if (!msgs || !msgs.length) {
            if (wbMessages) wbMessages.innerHTML = emptyHtml('开始对话吧');
            if (summary) appendSummaryBlock(summary);
            return;
        }
        wbMessages.innerHTML = '';
        for (var i = 0; i < msgs.length; i++) {
            var m = msgs[i];
            if (m.role === 'system') continue;
            if (m.role === 'tool') { appendToolResultBlock({ content: m.content || '' }); continue; }
            var toolCalls = null;
            if (m.tool_calls && typeof m.tool_calls === 'string') {
                try { toolCalls = JSON.parse(m.tool_calls); } catch(e) { toolCalls = null; }
            } else if (Array.isArray(m.tool_calls)) {
                toolCalls = m.tool_calls;
            }
            // assistant 消息无文本内容时跳过空气泡，仅渲染工具调用
            if (m.role === 'assistant' && (!m.content || !m.content.trim())) {
                if (toolCalls && toolCalls.length) {
                    for (var j = 0; j < toolCalls.length; j++) { appendToolCallBlock(toolCalls[j]); }
                }
                continue;
            }
            appendMessage(m.role, m.content, toolCalls, m.agent_name);
        }
        if (summary) {
            appendSummaryBlock(summary);
        }
        scrollToBottom();
    }

    function appendMessage(role, content, toolCalls, agentName) {
        var div = document.createElement('div');
        div.className = 'wb-message wb-message-' + role;
        var avatar = (role === 'user') ? 'U' : 'A';
        var label  = (role === 'user') ? '你' : (agentName || 'AI');
        div.innerHTML = '<div class="wb-message-avatar">' + avatar + '</div>' +
            '<div class="wb-message-body">' +
            '<div class="wb-message-role">' + escapeHtml(label) + '</div>' +
            '<div class="wb-message-bubble">' + escapeHtml(content || '') + '</div></div>';
        wbMessages.appendChild(div);
        if (toolCalls && toolCalls.length) {
            for (var i = 0; i < toolCalls.length; i++) { appendToolCallBlock(toolCalls[i]); }
        }
    }

    function appendStreamingMessage(agentName) {
        var div = document.createElement('div');
        div.className = 'wb-message wb-message-assistant';
        div.id = 'streaming-msg';
        var label = agentName || 'AI';
        div.innerHTML = '<div class="wb-message-avatar">A</div>' +
            '<div class="wb-message-body"><div class="wb-message-role">' + escapeHtml(label) + '</div>' +
            '<div class="wb-message-bubble"><span class="wb-typing">思考中</span></div></div>';
        wbMessages.appendChild(div);
        scrollToBottom();
        return div;
    }

    function appendToolCallBlock(data) {
        var div = document.createElement('div');
        div.className = 'wb-tool-call';
        var name = (data && data.name) ? data.name : (data && data.function && data.function.name) || '工具调用';
        // 兼容 SSE 推送（args）和数据库加载（arguments）两种键名
        var argsStr = data && (data.arguments || data.args);
        var hasArgs = false;
        if (argsStr) {
            if (typeof argsStr !== 'string') argsStr = JSON.stringify(argsStr);
            // 尝试格式化 JSON
            try { argsStr = JSON.stringify(JSON.parse(argsStr), null, 2); } catch(e) {}
            hasArgs = argsStr.length > 0;
        }
        var headerHtml = '<div class="wb-tool-call-header">' +
            '<svg class="wb-tool-call-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
            '<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>' +
            '<span class="wb-tool-call-name">' + escapeHtml(name) + '</span>';
        if (hasArgs) {
            headerHtml += '<svg class="wb-tool-call-toggle" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
                '<polyline points="6 9 12 15 18 9"/></svg>';
        }
        headerHtml += '</div>';
        div.innerHTML = headerHtml;
        if (hasArgs) {
            var argsDiv = document.createElement('div');
            argsDiv.className = 'wb-tool-call-args';
            argsDiv.textContent = argsStr;
            div.appendChild(argsDiv);
            div.querySelector('.wb-tool-call-header').addEventListener('click', function() {
                div.classList.toggle('wb-expanded');
            });
        }
        wbMessages.appendChild(div);
        scrollToBottom();
    }

    function appendToolResultBlock(data) {
        var div = document.createElement('div');
        var content = (data && data.content) || '';
        var isError = (data && data.is_error === 'true') || content.indexOf('[错误]') === 0;
        div.className = 'wb-tool-result' + (isError ? ' wb-tool-error' : '');
        var previewLen = isError ? 120 : 160;
        var previewText = content.replace(/\n/g, ' ').trim();
        var labelText = isError ? '工具调用失败' : (previewText.length > previewLen ? previewText.substring(0, previewLen) + '...' : previewText || '工具调用完成');
        var iconSvg = isError
            ? '<svg class="wb-tool-result-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>'
            : '<svg class="wb-tool-result-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';
        var headerHtml = '<div class="wb-tool-result-header">' +
            iconSvg +
            '<span class="wb-tool-result-label">' + escapeHtml(labelText) + '</span>';
        var showBody = !!content;
        if (showBody) {
            headerHtml += '<svg class="wb-tool-result-toggle" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
                '<polyline points="6 9 12 15 18 9"/></svg>';
        }
        headerHtml += '</div>';
        div.innerHTML = headerHtml;
        if (showBody) {
            var bodyDiv = document.createElement('div');
            bodyDiv.className = 'wb-tool-result-body';
            bodyDiv.textContent = content;
            div.appendChild(bodyDiv);
            div.querySelector('.wb-tool-result-header').addEventListener('click', function() {
                div.classList.toggle('wb-expanded');
            });
        }
        wbMessages.appendChild(div);
        scrollToBottom();
    }

    function appendSummaryBlock(text) {
        var div = document.createElement('div');
        div.className = 'wb-summary-block';
        var header = document.createElement('div');
        header.className = 'wb-summary-header';
        header.innerHTML = '<svg class="wb-summary-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
            '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/>' +
            '</svg><span>会话总结</span>';
        div.appendChild(header);
        var body = document.createElement('div');
        body.className = 'wb-summary-body';
        body.textContent = text;
        div.appendChild(body);
        wbMessages.appendChild(div);
        scrollToBottom();
    }

    // -- SSE 事件处理 --

    // 共享 SSE 事件处理函数（sendMessage 和 attachToRunning 共用）
    function handleSSEEvent(event, state) {
        if (event.seq !== undefined) lastSeq = event.seq;
        switch (event.type) {
            case 'reasoning':
                var rText = event.content || '';
                // 首次收到推理内容时，清除"思考中"，创建推理区域
                var rTyping = state.contentEl.querySelector('.wb-typing');
                if (rTyping) state.contentEl.removeChild(rTyping);
                var rBlock = state.contentEl.querySelector('.wb-reasoning');
                if (!rBlock) {
                    rBlock = document.createElement('div');
                    rBlock.className = 'wb-reasoning';
                    state.contentEl.appendChild(rBlock);
                }
                rBlock.appendChild(document.createTextNode(rText));
                scrollToBottom();
                break;
            case 'content':
                var chunk = event.content || '';
                state.fullContent += chunk;
                // 首次收到内容时清除"思考中"指示
                var typingEl = state.contentEl.querySelector('.wb-typing');
                if (typingEl) state.contentEl.removeChild(typingEl);

                // <think> 标签检测：流式 content 中可能包含 <think>...</think>
                var remaining = chunk;
                while (remaining.length > 0) {
                    if (state.inThinkTag) {
                        var closeIdx = remaining.indexOf('</think>');
                        if (closeIdx !== -1) {
                            state.thinkBuffer += remaining.substring(0, closeIdx);
                            remaining = remaining.substring(closeIdx + 8);
                            state.inThinkTag = false;
                            // 折叠推理区域
                            var rFinish = state.contentEl.querySelector('.wb-reasoning');
                            if (rFinish && !rFinish.classList.contains('wb-reasoning-done')) {
                                rFinish.classList.add('wb-reasoning-done');
                                rFinish.addEventListener('click', function() {
                                    this.classList.toggle('wb-reasoning-expanded');
                                });
                            }
                        } else {
                            state.thinkBuffer += remaining;
                            var trBlock = state.contentEl.querySelector('.wb-reasoning');
                            if (trBlock) trBlock.appendChild(document.createTextNode(remaining));
                            remaining = '';
                        }
                    } else {
                        var openIdx = remaining.indexOf('<think>');
                        if (openIdx !== -1) {
                            // <think> 之前的正常内容
                            var before = remaining.substring(0, openIdx);
                            if (before) {
                                // 收到正式内容时，折叠推理区域
                                var rDone2 = state.contentEl.querySelector('.wb-reasoning');
                                if (rDone2 && !rDone2.classList.contains('wb-reasoning-done')) {
                                    rDone2.classList.add('wb-reasoning-done');
                                    rDone2.addEventListener('click', function() {
                                        this.classList.toggle('wb-reasoning-expanded');
                                    });
                                }
                                state.contentEl.appendChild(document.createTextNode(before));
                            }
                            state.inThinkTag = true;
                            state.thinkBuffer = '';
                            remaining = remaining.substring(openIdx + 7);
                            // 创建推理区域
                            var trBlock2 = state.contentEl.querySelector('.wb-reasoning');
                            if (!trBlock2) {
                                trBlock2 = document.createElement('div');
                                trBlock2.className = 'wb-reasoning';
                                state.contentEl.appendChild(trBlock2);
                            }
                        } else {
                            // 收到正式内容时，折叠推理区域
                            var rDone = state.contentEl.querySelector('.wb-reasoning');
                            if (rDone && !rDone.classList.contains('wb-reasoning-done')) {
                                rDone.classList.add('wb-reasoning-done');
                                rDone.addEventListener('click', function() {
                                    this.classList.toggle('wb-reasoning-expanded');
                                });
                            }
                            state.contentEl.appendChild(document.createTextNode(remaining));
                            remaining = '';
                        }
                    }
                }
                scrollToBottom();
                break;
            case 'tool_call':
                appendToolCallBlock(event.data);
                if (event.data && event.data.name && event.data.name.indexOf('tasks_') === 0) {
                    pendingTaskRefresh = true;
                }
                break;
            case 'tool_result':
                appendToolResultBlock(event.data);
                state.streamDiv = appendStreamingMessage(state.agentName);
                state.contentEl = state.streamDiv.querySelector('.wb-message-bubble');
                state.fullContent = '';
                state.inThinkTag = false;
                state.thinkBuffer = '';
                if (pendingTaskRefresh && taskPanelVisible) {
                    pendingTaskRefresh = false;
                    scheduleTaskRefresh();
                }
                break;
            case 'error':
                var errTyping = state.contentEl.querySelector('.wb-typing');
                if (errTyping) state.contentEl.removeChild(errTyping);
                var errReasoning = state.contentEl.querySelector('.wb-reasoning');
                if (errReasoning) errReasoning.remove();
                state.contentEl.textContent = '错误: ' + (event.content || event.message || '未知错误');
                state.contentEl.classList.add('wb-message-error');
                state.streamDiv.setAttribute('data-has-error', '1');
                break;
            case 'sub_conv_start':
                if (event.data) updateSubConvBadge(event.data.conv_id, 'running', event.data.title, event.data.agent_name);
                break;
            case 'sub_conv_done':
                if (event.data) updateSubConvBadge(event.data.conv_id, 'done');
                break;
            case 'sub_conv_error':
                if (event.data) updateSubConvBadge(event.data.conv_id, 'error');
                break;
            case 'conv_switch':
                if (event.data && event.data.new_conv_id) {
                    currentConvId = event.data.new_conv_id;
                    WB_CONFIG.activeConvId = event.data.new_conv_id;
                    history.replaceState(null, '', '/admin/workbench?workspace_id=' + WB_CONFIG.workspaceId + '&conv_id=' + event.data.new_conv_id);
                    loadConversations();
                }
                break;
            case 'status':
                appendStatusNotice(event.content || '');
                break;
            case 'agent_info':
                if (event.data && event.data.agent_name) {
                    state.agentName = event.data.agent_name;
                    // 更新当前流式消息的标签
                    var roleEl = state.streamDiv ? state.streamDiv.querySelector('.wb-message-role') : null;
                    if (roleEl) roleEl.textContent = event.data.agent_name;
                }
                break;
            case 'conv_title_update':
                if (event.data && event.data.conv_id && event.data.title) {
                    var convItem = document.querySelector('.wb-conv-item[data-id="' + event.data.conv_id + '"] .wb-conv-title');
                    if (convItem) convItem.textContent = event.data.title;
                }
                break;
            case 'token_update':
                if (event.data && event.data.total_tokens) {
                    updateConvTokens(event.data.total_tokens);
                }
                break;
            case 'done':
                if (event.data && event.data.total_tokens) {
                    updateConvTokens(event.data.total_tokens);
                }
                break;
            case 'stopped':
                var stTyping = state.contentEl ? state.contentEl.querySelector('.wb-typing') : null;
                if (stTyping) stTyping.remove();
                var stReasoning = state.contentEl ? state.contentEl.querySelector('.wb-reasoning') : null;
                if (stReasoning) stReasoning.remove();
                if (state.contentEl) {
                    if (state.fullContent && state.fullContent.trim()) {
                        var stopMark = document.createElement('span');
                        stopMark.className = 'wb-message-stopped';
                        stopMark.textContent = ' [已停止]';
                        state.contentEl.appendChild(stopMark);
                    } else {
                        state.contentEl.textContent = '[会话已停止]';
                        state.contentEl.classList.add('wb-message-stopped');
                    }
                }
                activeTask = false;
                break;
        }
    }

    function appendStatusNotice(text) {
        if (!text) return;
        var div = document.createElement('div');
        div.className = 'wb-status-notice';
        div.textContent = text;
        wbMessages.appendChild(div);
        scrollToBottom();
    }

    // 更新/插入子会话状态徽章
    function updateSubConvBadge(convId, status, title, agentName) {
        if (!convId || !wbSessionList) return;
        subConvStatus[convId] = status;
        var item = wbSessionList.querySelector('[data-id="' + convId + '"]');
        if (!item) {
            // sub_conv_start 时动态插入节点
            if (status !== 'running') return;
            // 清除"暂无子会话"占位符
            var emptyEl = wbSessionList.querySelector('.wb-empty');
            if (emptyEl) emptyEl.remove();
            item = document.createElement('div');
            item.className = 'wb-session-item';
            item.setAttribute('data-id', convId);
            var displayTitle = title || agentName || '子会话';
            item.innerHTML =
                '<span class="wb-status-badge wb-status-running"></span>' +
                '<span class="wb-session-title">' + escapeHtml(displayTitle) + '</span>' +
                '<span class="wb-session-delete wb-conv-delete" data-id="' + convId + '">&times;</span>';
            item.addEventListener('click', function(e) {
                if (e.target.classList.contains('wb-conv-delete')) {
                    deleteConversation(parseInt(e.target.getAttribute('data-id'), 10));
                    return;
                }
                selectConversation(convId);
                if (wbSidebar) wbSidebar.classList.remove('wb-sidebar-open');
                if (wbSidebarOverlay) wbSidebarOverlay.classList.remove('active');
            });
            wbSessionList.appendChild(item);
            return;
        }
        var badge = item.querySelector('.wb-status-badge');
        if (!badge) {
            badge = document.createElement('span');
            item.insertBefore(badge, item.firstChild);
        }
        badge.className = 'wb-status-badge wb-status-' + status;
    }

    function finishStreaming(cleanlyDone) {
        sending = false;
        if (cleanlyDone) activeTask = false;
        setSendButtonState('idle');
        if (wbInput) wbInput.focus();
        var streamingEl = document.getElementById('streaming-msg');
        if (streamingEl) {
            // 有错误信息的气泡不清除
            if (streamingEl.getAttribute('data-has-error')) {
                streamingEl.removeAttribute('id');
            } else {
                var c = streamingEl.querySelector('.wb-message-bubble');
                if (c && (c.querySelector('.wb-typing') || !c.textContent.trim())) {
                    streamingEl.remove();
                } else if (streamingEl.id) {
                    streamingEl.removeAttribute('id');
                }
            }
        }
        if (taskPanelVisible) loadTasks();
        loadConversations();
    }

    // 建立 SSE 连接并处理事件流
    function connectSSE(state, body) {
        var convId = currentConvId;
        fetch('/admin/workbench/conversations/' + convId + '/send', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCSRF() },
            body: JSON.stringify(body)
        }).then(function(response) {
            if (!response.ok) {
                response.text().then(function(text) {
                    var msg = text.trim() || '请求失败 ' + response.status;
                    if (response.status === 400 || response.status === 404) {
                        // 有消息的请求被拒绝时显示错误，空消息重连场景静默完成
                        if (body.message) {
                            state.contentEl.textContent = '错误: ' + msg;
                            state.contentEl.classList.add('wb-message-error');
                            state.streamDiv.setAttribute('data-has-error', '1');
                        }
                        finishStreaming(true);
                    } else if (response.status === 409) {
                        state.contentEl.textContent = '错误: ' + msg;
                        finishStreaming(true);
                    } else {
                        state.contentEl.textContent = '错误: ' + msg;
                        finishStreaming(false);
                    }
                });
                return;
            }
            var reader = response.body.getReader();
            var decoder = new TextDecoder();
            var buf = '';

            function read() {
                reader.read().then(function(result) {
                    if (result.done) { finishStreaming(false); return; }
                    buf += decoder.decode(result.value, { stream: true });
                    var lines = buf.split('\n');
                    buf = lines.pop();
                    for (var i = 0; i < lines.length; i++) {
                        var line = lines[i];
                        if (line.indexOf('data: ') === 0) {
                            var raw = line.substring(6).trim();
                            if (raw === '[DONE]') { finishStreaming(true); return; }
                            try { handleSSEEvent(JSON.parse(raw), state); } catch(e) {}
                        }
                    }
                    read();
                }).catch(function() {
                    if (activeTask && currentConvId === convId) {
                        scheduleReconnect(state, convId);
                    } else {
                        finishStreaming(false);
                    }
                });
            }
            read();
        }).catch(function() {
            if (activeTask && currentConvId === convId) {
                scheduleReconnect(state, convId);
            } else {
                finishStreaming(false);
            }
        });
    }

    // 断线后延迟重连
    function scheduleReconnect(state, convId) {
        if (reconnectTimer) clearTimeout(reconnectTimer);
        reconnectTimer = setTimeout(function() {
            reconnectTimer = null;
            if (!activeTask || currentConvId !== convId) return;
            state.streamDiv = appendStreamingMessage();
            state.contentEl = state.streamDiv.querySelector('.wb-message-bubble');
            state.contentEl.textContent = '重连中...';
            state.fullContent = '';
            state.inThinkTag = false;
            state.thinkBuffer = '';
            var body = { message: '', workspace_id: WB_CONFIG.workspaceId || 0, last_seq: lastSeq };
            connectSSE(state, body);
        }, 3000);
    }

    // -- 发送消息 --

    function sendMessage() {
        var msg = wbInput ? wbInput.value.trim() : '';
        if (!msg || sending) return;
        if (!WB_CONFIG.workspaceId) return;

        if (!currentConvId) {
            // 没有会话时自动创建
            var agentId = wbSelAgent && wbSelAgent.value ? parseInt(wbSelAgent.value, 10) : 0;
            var title = '新会话';
            apiRequest('/admin/workbench/conversations', {
                method: 'POST',
                body: JSON.stringify({
                    workspace_id: WB_CONFIG.workspaceId,
                    agent_id: agentId,
                    title: title,
                    conv_type: 'main'
                })
            }).then(function(resp) {
                if (resp.data && resp.data.id) {
                    currentConvId = resp.data.id;
                    WB_CONFIG.activeConvId = resp.data.id;
                    history.replaceState(null, '', '/admin/workbench?workspace_id=' + WB_CONFIG.workspaceId + '&conv_id=' + resp.data.id);
                    loadConversations();
                    doSend(msg);
                } else {
                    alert(resp.message || '创建会话失败');
                }
            }).catch(function() { alert('网络错误，请重试'); });
            return;
        }

        doSend(msg);
    }

    function doSend(msg) {
        sending = true;
        activeTask = true;
        if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
        setSendButtonState('sending');
        if (wbInput) { wbInput.value = ''; wbInput.style.height = 'auto'; }

        var emptyEl = wbMessages ? wbMessages.querySelector('.wb-empty') : null;
        if (emptyEl) emptyEl.remove();

        appendMessage('user', msg);
        scrollToBottom();

        // 获取当前选中智能体的 title 作为初始值
        var initAgentName = '';
        if (wbSelAgent && wbSelAgent.value && WB_CONFIG.agents) {
            var selId = parseInt(wbSelAgent.value, 10);
            for (var i = 0; i < WB_CONFIG.agents.length; i++) {
                if (WB_CONFIG.agents[i].id === selId) {
                    var ag = WB_CONFIG.agents[i];
                    initAgentName = (ag.title && ag.name) ? ag.title + '-' + ag.name : (ag.title || ag.name || '');
                    break;
                }
            }
        }

        var streamDiv = appendStreamingMessage(initAgentName);
        var state = {
            streamDiv: streamDiv,
            contentEl: streamDiv.querySelector('.wb-message-bubble'),
            fullContent: '',
            inThinkTag: false,
            thinkBuffer: '',
            agentName: initAgentName
        };

        lastSeq = 0;
        var body = { message: msg, workspace_id: WB_CONFIG.workspaceId || 0, last_seq: 0 };
        if (wbSelAgent && wbSelAgent.value) body.agent_id = parseInt(wbSelAgent.value, 10);
        connectSSE(state, body);
    }

    // 订阅/重连已有进行中的任务（不发送新消息）
    function attachToRunning(convId) {
        if (!convId || !WB_CONFIG.workspaceId) return;
        if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
        activeTask = true;
        sending = true;
        setSendButtonState('sending');

        var streamDiv = appendStreamingMessage();
        var state = {
            streamDiv: streamDiv,
            contentEl: streamDiv.querySelector('.wb-message-bubble'),
            fullContent: '',
            inThinkTag: false,
            thinkBuffer: '',
            agentName: ''
        };

        var body = { message: '', workspace_id: WB_CONFIG.workspaceId || 0, last_seq: lastSeq };
        connectSSE(state, body);
    }

    if (wbBtnSend) wbBtnSend.addEventListener('click', function() {
        if (activeTask) { stopConversation(); } else { sendMessage(); }
    });

    if (wbInput) {
        wbInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
        });
        wbInput.addEventListener('input', function() {
            this.style.height = 'auto';
            this.style.height = Math.min(this.scrollHeight, 150) + 'px';
        });
    }

    // -- 智能体选择时自动同步模型 --

    function syncModelFromAgent() {
        if (!wbSelAgent || !wbSelModel) return;
        var opt = wbSelAgent.options[wbSelAgent.selectedIndex];
        if (!opt || !opt.value) return;
        var modelMode = opt.getAttribute('data-model-mode') || 'auto';
        var agentModelId = opt.getAttribute('data-model-id');

        if (modelMode === 'auto' || modelMode === 'default') {
            wbSelModel.value = modelMode;
        } else if (agentModelId && agentModelId !== '0') {
            wbSelModel.value = agentModelId;
            if (wbSelModel.value !== agentModelId) wbSelModel.value = 'auto';
        } else {
            wbSelModel.value = 'auto';
        }
    }

    if (wbSelAgent) wbSelAgent.addEventListener('change', syncModelFromAgent);

    // -- 初始化 --

    function init() {
        updateToolbarInfo();

        if (WB_CONFIG.activeConvId > 0) {
            currentConvId = WB_CONFIG.activeConvId;
        }

        if (WB_CONFIG.workspaceId) {
            loadConversations();
            if (wbInput) {
                wbInput.disabled = false;
                wbInput.placeholder = '输入消息，Enter 发送，Shift+Enter 换行...';
            }
            if (wbBtnSend) setSendButtonState('idle');
            if (wbToggleTasks) wbToggleTasks.style.display = '';
            if (wbToggleDocs) wbToggleDocs.style.display = '';
            if (wbToggleProjDocs) wbToggleProjDocs.style.display = WB_CONFIG.hasProject ? '' : 'none';
            if (wbToggleFiles) wbToggleFiles.style.display = WB_CONFIG.hasProject ? '' : 'none';
            var savedPanel = '';
            try { savedPanel = localStorage.getItem('wb_right_panel') || ''; } catch(e) {}
            if (savedPanel === 'tasks') showTaskPanel();
            else if (savedPanel === 'docs') showDocsPanel();
            else if (savedPanel === 'proj-docs' && WB_CONFIG.hasProject) showProjDocsPanel();
            else if (savedPanel === 'files' && WB_CONFIG.hasProject) showFilesPanel();
        } else {
            if (wbInput) wbInput.placeholder = '请先在左侧选择工作区...';
        }

        if (WB_CONFIG.agentId && wbSelAgent) {
            wbSelAgent.value = WB_CONFIG.agentId;
            syncModelFromAgent();
        }

        if (WB_CONFIG.activeConvId > 0) {
            loadMessages(WB_CONFIG.activeConvId).then(function() {
                if (WB_CONFIG.isRunning) {
                    attachToRunning(WB_CONFIG.activeConvId);
                }
            });
        }
    }

    init();
})();
