/* ========================================
   AgentFlow — 原生 JavaScript
   主题切换 · 移动菜单 · 表单 · 删除
   ======================================== */

document.addEventListener('DOMContentLoaded', function () {
    initTheme();
    initMobileMenu();
    initSidebar();
    initDropdowns();
    initForms();
    initDeleteButtons();
    initCaptcha();
    initTabs();
});

/* --- 主题切换 --- */
function initTheme() {
    var saved = localStorage.getItem('theme');
    var prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    var dark = saved ? saved === 'dark' : prefersDark;

    applyTheme(dark);

    // 监听系统主题变化
    window.matchMedia('(prefers-color-scheme: dark)')
        .addEventListener('change', function (e) {
            if (!localStorage.getItem('theme')) {
                applyTheme(e.matches);
            }
        });

    // 绑定切换按钮
    var btn = document.getElementById('theme-toggle-btn');
    if (btn) {
        btn.addEventListener('click', function () {
            var isDark = document.documentElement.getAttribute('data-theme') === 'dark';
            var newTheme = !isDark;
            localStorage.setItem('theme', newTheme ? 'dark' : 'light');
            applyTheme(newTheme);
        });
    }
}

function applyTheme(dark) {
    document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light');
}

/* --- 移动端菜单 --- */
function initMobileMenu() {
    var btn = document.getElementById('mobile-menu-btn');
    var menu = document.getElementById('mobile-menu');
    if (!btn || !menu) return;

    btn.addEventListener('click', function (e) {
        e.stopPropagation();
        menu.classList.toggle('open');
    });

    // 点击外部关闭
    document.addEventListener('click', function (e) {
        if (!menu.contains(e.target) && !btn.contains(e.target)) {
            menu.classList.remove('open');
        }
    });
}

/* --- 移动端侧边栏 --- */
function initSidebar() {
    var sidebar = document.getElementById('admin-sidebar');
    var overlay = document.getElementById('sidebar-overlay');
    var btn = document.getElementById('mobile-menu-btn');
    if (!sidebar || !btn) return;

    btn.addEventListener('click', function (e) {
        e.stopPropagation();
        sidebar.classList.toggle('sidebar-open');
        if (overlay) overlay.classList.toggle('active');
    });

    if (overlay) {
        overlay.addEventListener('click', function () {
            sidebar.classList.remove('sidebar-open');
            overlay.classList.remove('active');
        });
    }

    // 点击侧边栏链接后自动关闭
    sidebar.querySelectorAll('a').forEach(function (link) {
        link.addEventListener('click', function () {
            sidebar.classList.remove('sidebar-open');
            if (overlay) overlay.classList.remove('active');
        });
    });
}

/* --- 下拉菜单 --- */
function initDropdowns() {
    document.querySelectorAll('[data-dropdown]').forEach(function (toggle) {
        var dropdown = toggle.closest('.dropdown');
        if (!dropdown) return;

        toggle.addEventListener('click', function (e) {
            e.stopPropagation();
            // 关闭其他已打开的下拉
            document.querySelectorAll('.dropdown.open').forEach(function (d) {
                if (d !== dropdown) d.classList.remove('open');
            });
            dropdown.classList.toggle('open');
        });
    });

    // 点击外部关闭所有下拉
    document.addEventListener('click', function () {
        document.querySelectorAll('.dropdown.open').forEach(function (d) {
            d.classList.remove('open');
        });
    });
}

/* --- 表单提交状态 --- */
function initForms() {
    var forms = document.querySelectorAll('form[data-submit-text]');
    forms.forEach(function (form) {
        form.addEventListener('submit', function () {
            var submitBtn = form.querySelector('button[type="submit"]');
            if (submitBtn && !submitBtn.disabled) {
                submitBtn.disabled = true;
                submitBtn.textContent = form.getAttribute('data-submit-text') || '提交中...';
            }
        });
    });
}

/* --- 删除按钮（fetch + 确认） --- */
function initDeleteButtons() {
    document.querySelectorAll('[data-delete-url]').forEach(function (btn) {
        btn.addEventListener('click', function () {
            var url = btn.getAttribute('data-delete-url');
            var confirmMsg = btn.getAttribute('data-confirm') || '确定要删除吗？';
            var targetId = btn.getAttribute('data-target');

            if (!confirm(confirmMsg)) return;

            // 从 Cookie 读取 CSRF Token
            var csrfToken = '';
            var match = document.cookie.match(/(?:^|;\s*)_csrf_token=([^;]*)/);
            if (match) csrfToken = match[1];

            fetch(url, {
                method: 'DELETE',
                headers: { 'X-CSRF-Token': csrfToken, 'X-Requested-With': 'fetch' }
            })
                .then(function (resp) {
                    if (resp.ok && targetId) {
                        var row = document.getElementById(targetId);
                        if (row) {
                            row.classList.add('fade-out');
                            setTimeout(function () { row.remove(); }, 300);
                        }
                    } else if (!resp.ok) {
                        alert('删除失败，请稍后重试');
                    }
                })
                .catch(function () {
                    alert('网络错误，请稍后重试');
                });
        });
    });
}

/* --- 验证码刷新 --- */
function initCaptcha() {
    var refreshBtn = document.getElementById('captcha-refresh');
    if (!refreshBtn) return;

    refreshBtn.addEventListener('click', function () {
        fetch('/auth/captcha')
            .then(function (resp) { return resp.json(); })
            .then(function (json) {
                var data = json.data;
                if (data && data.id && data.svg) {
                    document.getElementById('captcha-id').value = data.id;
                    document.getElementById('captcha-svg').innerHTML = data.svg;
                }
            })
            .catch(function () {});
    });
}

/* --- Tab 切换 --- */
function initTabs() {
    var tabBtns = document.querySelectorAll('.tab-btn[data-tab], .tab-nav-btn[data-tab]');
    if (!tabBtns.length) return;

    tabBtns.forEach(function (btn) {
        btn.addEventListener('click', function () {
            var tabName = btn.getAttribute('data-tab');
            var container = btn.closest('form') || document;

            // 切换按钮激活状态
            container.querySelectorAll('.tab-btn[data-tab], .tab-nav-btn[data-tab]').forEach(function (b) {
                b.classList.remove('tab-active');
            });
            btn.classList.add('tab-active');

            // 切换内容面板
            container.querySelectorAll('.tab-pane[data-tab-content]').forEach(function (pane) {
                pane.classList.remove('tab-pane-active');
            });
            var target = container.querySelector('.tab-pane[data-tab-content="' + tabName + '"]');
            if (target) target.classList.add('tab-pane-active');
        });
    });
}
