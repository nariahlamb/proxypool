// ProxyPool 前端脚本（原生 JS，无外部依赖）
// 功能：导航切换、复制、订阅链接渲染（部署前缀自动适配）、任务触发（admin_token 鉴权）

// 已知根路由第一段（用于从前端 URL 推断部署前缀）
var ROOT_ROUTES = ["clash", "surge", "shadowrocket", "loon", "quanx", "v2rayn", "best",
    "static", "health", "task", "ss", "ssr", "vmess", "trojan", "vless", "sip002",
    "link", "debug", "bestProxyIp", "bestCfProxyIp", "bestCfProxyIpTop20",
    "bestCfProxyIpIsp", "bestCfProxyDomainTop20", "bestCfProxySub", "bestIpKr"];

// 获取部署前缀：服务端注入非空前缀时直接采用；否则回退到浏览器地址推断。
// 外部前缀由反向代理如 nginx "/show/ --> /" 映射，应用在 nginx 剥离后收到的是根路径，
// 但浏览器地址仍是 "/show/..."，此时需按 location 推断出前缀（本地根路径则推断为空）。
function getBasePath() {
    if (window.PROXYPOOL_BASE_PATH) return window.PROXYPOOL_BASE_PATH;
    var segs = location.pathname.split("/"); // ["", "show", "clash", ...]
    if (segs.length >= 2 && segs[1] && ROOT_ROUTES.indexOf(segs[1]) === -1) {
        return "/" + segs[1];
    }
    return "";
}

// 拼接完整订阅 URL：origin + 部署前缀 + 相对路径
function getSubURL(rel) {
    if (!rel) return "";
    var url = location.origin + getBasePath() + (rel.charAt(0) === "/" ? rel : "/" + rel);
    // best 优选订阅鉴权：链接自动拼已保存的 best_token（无则不拼，不弹窗；
    // 输入提示仅在用户主动点击复制时由 ensureBestToken 触发）
    if (rel.indexOf("/best") === 0) {
        var token = localStorage.getItem("proxypool_best_token") || "";
        if (token) url += (url.indexOf("?") >= 0 ? "&" : "?") + "token=" + encodeURIComponent(token);
    }
    return url;
}

// ensureBestToken 复制 /best* 链接前确保有 token（无则提示输入一次并存 localStorage）
function ensureBestToken() {
    var token = localStorage.getItem("proxypool_best_token") || "";
    if (!token) {
        token = window.prompt("请输入 best_token（config.yaml 中配置，用于优选订阅鉴权）：");
        if (token) localStorage.setItem("proxypool_best_token", token);
    }
    return token;
}

document.addEventListener("DOMContentLoaded", function () {
    initPageChrome();
});

// 页面级“外壳”初始化（每次整页加载只跑一次）：主题、导航、软导航绑定
function initPageChrome() {
    applyTheme(currentTheme());
    watchSystemTheme();
    var themeBtn = document.getElementById("theme-toggle");
    if (themeBtn && !themeBtn.__bound) { themeBtn.__bound = true; themeBtn.addEventListener("click", toggleTheme); }

    // 移动端导航菜单切换（Tailwind 重构版：toggle hidden）
    var burger = document.getElementById("nav-burger");
    if (burger && !burger.__bound) {
        burger.__bound = true;
        burger.addEventListener("click", function () {
            var menu = document.getElementById("nav-menu");
            if (menu) menu.classList.toggle("hidden");
        });
    }
    // 首页品牌链接跳转部署前缀首页
    var brand = document.getElementById("brand-home");
    if (brand) brand.setAttribute("href", getBasePath() + "/");

    // 软导航：拦截同源页面跳转，fetch + 替换主内容，避免整页刷新
    bindSoftNav();
    window.addEventListener("popstate", function () { softNavigate(location.href, false); });

    initPageFeatures();
}

// 每次主内容渲染后调用：填充订阅链接、初始化优选生成器/搜索等
function initPageFeatures() {
    var subs = document.querySelectorAll("[data-sub-path]");
    for (var i = 0; i < subs.length; i++) {
        var el = subs[i];
        var rel = el.getAttribute("data-sub-path");
        var url = getSubURL(rel);
        if (el.tagName === "TD" || el.hasAttribute("data-text")) {
            el.textContent = url;
        } else if (el.tagName === "A" && el.hasAttribute("data-install-scheme")) {
            var scheme = el.getAttribute("data-install-scheme");
            el.setAttribute("href", scheme + ":///install-config?url=" + encodeURIComponent(url));
        } else {
            el.setAttribute("data-copy", url);
        }
    }
    if (document.getElementById("gen-client")) refreshBestGen();
    initSubSearch();
    applyTheme(currentTheme()); // 同步新页面的主题色 meta
}

// 绑定软导航：拦截同源、非新标签页、非自定义 scheme 的 <a> 点击
function bindSoftNav() {
    document.addEventListener("click", function (e) {
        var a = e.target && e.target.closest ? e.target.closest("a") : null;
        if (!a) return;
        if (a.target === "_blank") return;
        var href = a.getAttribute("href");
        if (!href || href.charAt(0) === "#" || /^(javascript:|mailto:|tel:|clash:|surge3:|about:)/i.test(href)) return;
        var abs;
        try { abs = new URL(href, location.href); } catch (err) { return; }
        if (abs.origin !== location.origin) return;
        if (abs.href === location.href) { e.preventDefault(); return; }
        e.preventDefault();
        softNavigate(abs.href, true);
    });
}

// 软导航：fetch 目标页 → 淡出旧内容 → 替换主内容 → 新内容自下而上淡入
function softNavigate(url, push) {
    var main = document.getElementById("main-content");
    if (!main) { location.assign(url); return; }
    if (push) history.pushState({}, "", url);
    fetch(url).then(function (r) { return r.text(); }).then(function (html) {
        var doc = new DOMParser().parseFromString(html, "text/html");
        var newMain = doc.querySelector("#main-content");
        if (!newMain) { location.assign(url); return; }
        applyScriptGlobals(doc);
        // 清除上一次导航留下的填充动画，避免其 fill 影响后续（否则可能把内容盖回透明）
        try { main.getAnimations().forEach(function (a) { a.cancel(); }); } catch (e) {}
        var leave = main.animate(
            [{ opacity: 1, transform: "translateY(0)" }, { opacity: 0, transform: "translateY(-8px)" }],
            { duration: 150, easing: "ease-out", fill: "forwards" }
        );
        leave.finished.then(function () {
            main.innerHTML = newMain.innerHTML;
            if (doc.title) document.title = doc.title;
            setNavActive(url);
            window.scrollTo(0, 0);
            initPageFeatures();
            main.animate(
                [{ opacity: 0, transform: "translateY(28px)" }, { opacity: 1, transform: "translateY(0)" }],
                // fill:both 让新页淡入完成后保持可见，避免旧淡出的 fill:forwards 把它盖回透明
                { duration: 320, easing: "cubic-bezier(0.4, 0, 0.2, 1)", fill: "both" }
            );
        });
    }).catch(function () { location.assign(url); });
}

// 提取目标页内联脚本里的页面级全局（如 best 生成器数据），innerHTML 替换不会执行 <script>
function applyScriptGlobals(doc) {
    var scripts = doc.querySelectorAll("script");
    for (var i = 0; i < scripts.length; i++) {
        var text = scripts[i].textContent || "";
        if (text.indexOf("BEST_GEN_COUNTRIES") >= 0 || text.indexOf("BEST_GEN_CLIENTS") >= 0) {
            try { (0, eval)(text); } catch (e) {}
        }
    }
}

// 更新导航当前激活项（服务端只在整页加载时渲染 active 高亮）
function setNavActive(url) {
    var u = new URL(url, location.href);
    var base = getBasePath();
    var rest = u.pathname.slice(base.length).replace(/^\//, "");
    var route = rest.split("/")[0] || "";
    var navMenu = document.getElementById("nav-menu");
    if (!navMenu) return;
    var links = navMenu.querySelectorAll("a");
    var act = ["bg-indigo-50", "text-indigo-700", "dark:bg-indigo-500/10", "dark:text-indigo-300"];
    for (var i = 0; i < links.length; i++) {
        var linkHref = links[i].getAttribute("href");
        var active = !!route && linkHref === route;
        for (var j = 0; j < act.length; j++) links[i].classList.toggle(act[j], active);
    }
}

// 主题切换（三态：dark / light / 跟随系统）。状态存 localStorage('proxypool_theme')，
// 未手动选择时跟随系统 prefers-color-scheme（含实时监听）。
var THEME_ICONS = {
    dark: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor" class="w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z"/></svg>`,
    light: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor" class="w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z"/></svg>`,
    system: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor" class="w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3a9 9 0 1 0 9 9c0-4.97-4.03-9-9-9Zm0 2a7 7 0 0 1 7 7c0 3.86-3.14 7-7 7V5Z"/></svg>`
};

function currentTheme() {
    return localStorage.getItem("proxypool_theme") || "system";
}

function applyTheme(t) {
    var dark = t === "dark" || (t !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.classList.toggle("dark", dark);
    var icon = document.getElementById("theme-icon");
    if (icon) icon.innerHTML = THEME_ICONS[t] || THEME_ICONS.system;
    // 移动端浏览器状态栏/主题色匹配页面背景
    var mc = document.querySelector('meta[name="theme-color"]');
    if (mc) mc.setAttribute("content", dark ? "#020617" : "#f6f7fb");
}

// 切换：dark → light → system → dark 循环
function toggleTheme() {
    var t = currentTheme();
    var next = t === "dark" ? "light" : (t === "light" ? "system" : "dark");
    localStorage.setItem("proxypool_theme", next);
    applyTheme(next);
    showTip(next === "system" ? "已切换：跟随系统" : (next === "dark" ? "已切换：夜间模式" : "已切换：日间模式"));
}

// 未手动选择时跟随系统主题变化
function watchSystemTheme() {
    if (!window.matchMedia) return;
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
        if (currentTheme() === "system") applyTheme("system");
    });
}

// 订阅表搜索过滤（subscription-search → subscription-row）
function initSubSearch() {
    var input = document.getElementById("subscription-search");
    if (!input) return;
    var rows = document.querySelectorAll(".subscription-row");
    input.addEventListener("input", function () {
        var kw = input.value.trim().toLowerCase();
        for (var i = 0; i < rows.length; i++) {
            var name = (rows[i].getAttribute("data-name") || "").toLowerCase();
            rows[i].style.display = name.indexOf(kw) >= 0 ? "" : "none";
        }
    });
}

// 复制文本：优先 navigator.clipboard，降级 execCommand
function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
        return navigator.clipboard.writeText(text).then(function () { return true; });
    }
    var t = document.createElement("textarea");
    t.value = text;
    t.style.position = "fixed";
    t.style.opacity = "0";
    document.body.appendChild(t);
    t.select();
    var ok = false;
    try { ok = document.execCommand("copy"); } catch (e) { ok = false; }
    document.body.removeChild(t);
    return Promise.resolve(ok);
}

// 复制/操作反馈 toast（Tailwind 版，aria-live 供读屏播报）
function showTip(msg) {
    var wrap = document.createElement("div");
    wrap.className = "fixed inset-0 z-[100] flex items-center justify-center pointer-events-none px-6";
    wrap.setAttribute("role", "status");
    wrap.setAttribute("aria-live", "polite");
    var inner = document.createElement("div");
    inner.className = "bg-slate-900/90 dark:bg-slate-100/90 text-white dark:text-slate-900 rounded-xl px-5 py-4 shadow-lg text-sm text-center max-w-[80vw]";
    inner.innerHTML = "<div class='text-3xl mb-1 leading-none'>✔</div>" + msg;
    wrap.appendChild(inner);
    document.body.appendChild(wrap);
    setTimeout(function () { wrap.remove(); }, 1800);
}

// 复制按钮：从上一单元格（td[data-sub-path]）取相对路径拼接完整 URL
function onCopy(e) {
    var cell = e.parentNode.previousElementSibling;
    var rel = cell ? cell.getAttribute("data-sub-path") : "";
    if (rel && rel.indexOf("/best") === 0) ensureBestToken();
    copyText(getSubURL(rel)).then(function (ok) {
        showTip(ok ? "复制成功" : "复制失败");
    });
}

// 复制按钮：从 data-sub-path 拼接完整 URL
function onCopyThis(e) {
    var rel = e.getAttribute("data-sub-path");
    if (rel && rel.indexOf("/best") === 0) ensureBestToken();
    copyText(getSubURL(rel)).then(function (ok) {
        showTip(ok ? "复制成功" : "复制失败");
    });
}

// 动态生成器：端点 × 客户端 × 协议 × 落地国家 → /{endpoint}/{client}{Protocol}?d={country}&...
// 三维联动：协议候选 = 客户端支持 ∩ 国家配置（BEST_GEN_CLIENTS × BEST_GEN_COUNTRIES）
var BEST_GEN_PROTO_ORDER = ["vless", "vmess", "trojan", "anytls"];

// 各端点支持的筛选参数（对照 api/router.go 路由）
var BEST_GEN_ENDPOINTS = {
    "bestProxyIp":            { d: true, c: true, limit: true, random: true, cdn: true, ipv6: true, sort: true },
    "bestCfProxyIp":          { d: true, c: false, limit: false, random: false, cdn: false, ipv6: true },
    "bestCfProxyIpTop20":     { d: true, c: false, limit: false, random: false, cdn: false, ipv6: true },
    "bestCfProxyIpIsp":       { d: true, c: false, limit: false, random: false, cdn: false, ipv6: true },
    "bestCfProxyDomainTop20": { d: true, c: false, limit: false, random: false, cdn: false, ipv6: true },
    "bestCfProxySub":         { d: true, c: false, limit: false, random: false, cdn: false, ipv6: true, sub: true },
    "bestIpKr":               { d: false, c: true, limit: true, random: true, cdn: true, ipv6: true, sort: true },
};

function setGenEnabled(id, on) {
    var el = document.getElementById(id);
    if (el) el.disabled = !on;
}

// 客户端 × 国家 交集：返回该组合可用的协议（按 BEST_GEN_PROTO_ORDER 顺序）
function bestGenProtocols(client, country) {
    var cs = (window.BEST_GEN_CLIENTS || {})[client] || [];
    var cs2 = (window.BEST_GEN_COUNTRIES || {})[country] || [];
    var out = [];
    for (var i = 0; i < BEST_GEN_PROTO_ORDER.length; i++) {
        var p = BEST_GEN_PROTO_ORDER[i];
        if (cs.indexOf(p) >= 0 && cs2.indexOf(p) >= 0) out.push(p);
    }
    return out;
}

function bestGenURL() {
    var ep = document.getElementById("gen-endpoint").value;
    var client = document.getElementById("gen-client").value;
    var proto = document.getElementById("gen-protocol").value;
    var caps = BEST_GEN_ENDPOINTS[ep] || {};
    if (!ep || !client || !proto) return "";
    var cap = proto.charAt(0).toUpperCase() + proto.slice(1);
    var url = "/" + ep + "/" + client + cap;
    var q = [];
    if (caps.d) {
        var country = document.getElementById("gen-country").value;
        if (country) q.push("d=" + country);
    }
    if (caps.c) {
        var c = document.getElementById("gen-c").value;
        if (c) q.push("c=" + encodeURIComponent(c));
    }
    if (caps.sub) {
        var sub = document.getElementById("gen-sub").value;
        if (sub) q.push("sub=" + encodeURIComponent(sub));
    }
    if (caps.limit) {
        var limit = document.getElementById("gen-limit").value;
        if (limit) q.push("limit=" + encodeURIComponent(limit));
    }
    if (caps.random && document.getElementById("gen-random").checked) q.push("random=true");
    if (caps.sort) {
        var sort = document.getElementById("gen-sort").value;
        if (sort) q.push("sort=" + sort);
    }
    if (caps.cdn && document.getElementById("gen-cdn").checked) q.push("cdn=true");
    if (caps.ipv6) {
        var ipv6 = document.getElementById("gen-ipv6").value;
        if (ipv6) q.push("ipv6=" + ipv6);
    }
    if (q.length) url += "?" + q.join("&");
    return getSubURL(url);
}

function refreshBestGen() {
    var ep = document.getElementById("gen-endpoint").value;
    var caps = BEST_GEN_ENDPOINTS[ep] || {};
    setGenEnabled("gen-country", caps.d);
    setGenEnabled("gen-c", caps.c);
    setGenEnabled("gen-sub", caps.sub);
    setGenEnabled("gen-limit", caps.limit);
    setGenEnabled("gen-random", caps.random);
    setGenEnabled("gen-sort", caps.sort);
    setGenEnabled("gen-cdn", caps.cdn);
    setGenEnabled("gen-ipv6", caps.ipv6);

    var client = document.getElementById("gen-client").value;
    var protoSel = document.getElementById("gen-protocol");
    var cur = protoSel.value;

    // 1. 协议 → 国家候选：只保留配置了当前协议的国家（按模板注入顺序）
    var countrySel = document.getElementById("gen-country");
    var protoForCountry = window.BEST_GEN_COUNTRIES || {};
    var countryOpts = countrySel.querySelectorAll("option");
    for (var i = 0; i < countryOpts.length; i++) {
        var ps = protoForCountry[countryOpts[i].value] || [];
        var hide = cur && ps.indexOf(cur) < 0;
        countryOpts[i].style.display = hide ? "none" : "";
        countryOpts[i].disabled = hide;
    }
    // 当前国家不可用则回退到第一个可见项
    if (countrySel.selectedOptions.length && countrySel.selectedOptions[0].disabled) {
        for (var j = 0; j < countryOpts.length; j++) {
            if (!countryOpts[j].disabled) { countrySel.value = countryOpts[j].value; break; }
        }
    }
    var country = caps.d ? countrySel.value : "KR";

    // 2. 国家 → 协议候选：客户端支持 ∩ 国家配置
    var protos = bestGenProtocols(client, country);
    protoSel.innerHTML = "";
    for (var k = 0; k < protos.length; k++) {
        var o = document.createElement("option");
        o.value = protos[k];
        o.textContent = protos[k];
        protoSel.appendChild(o);
    }
    if (protos.length) {
        var idx = protos.indexOf(cur);
        protoSel.value = (idx >= 0 ? cur : protos[0]);
    }

    var url = bestGenURL();
    document.getElementById("gen-url").value = url;
    return url;
}

// 复制动态生成的订阅链接（best 鉴权：先 ensureBestToken）
function copyBestGen(btn) {
    ensureBestToken();
    var ep = document.getElementById("gen-endpoint").value;
    var url = bestGenURL();
    if (!document.getElementById("gen-protocol").value) {
        showTip("该客户端+国家组合无可用的协议，请调整选择");
        return;
    }
    if (!url) { showTip("请先选择客户端/协议/国家"); return; }
    if (ep === "bestCfProxySub" && !document.getElementById("gen-sub").value) {
        showTip("bestCfProxySub 需填写 sub 订阅源链接");
        document.getElementById("gen-sub").focus();
        return;
    }
    copyText(url).then(function (ok) {
        showTip(ok ? "复制成功" : "复制失败");
    });
}

// 任务触发（admin_token 鉴权）：token 存 localStorage，请求 /task/xxx?token=
function runTask(name) {
    var token = localStorage.getItem("proxypool_admin_token") || "";
    if (!token) {
        token = window.prompt("请输入管理员 Token（配置中的 admin_token）：");
        if (!token) return;
        localStorage.setItem("proxypool_admin_token", token);
    }
    var url = getBasePath() + "/task/" + name + "?token=" + encodeURIComponent(token);
    fetch(url)
        .then(function (r) { return r.text().then(function (t) { return { status: r.status, text: t }; }); })
        .then(function (res) {
            if (res.status === 200) {
                showTip("任务已触发：" + name);
            } else if (res.status === 401) {
                localStorage.removeItem("proxypool_admin_token");
                showTip("Token 错误，请重试");
            } else {
                showTip("触发失败：" + res.text);
            }
        })
        .catch(function () { showTip("请求失败"); });
}

// 清除已保存的 Token
function clearToken() {
    localStorage.removeItem("proxypool_admin_token");
    showTip("已清除 Token");
}
