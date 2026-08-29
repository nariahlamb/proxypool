// ProxyPool 前端脚本（原生 JS，无外部依赖）
// 功能：导航切换、复制、订阅链接渲染（部署前缀自动适配）、任务触发（admin_token 鉴权）

// 已知根路由第一段（用于从前端 URL 推断部署前缀）
var ROOT_ROUTES = ["clash", "surge", "shadowrocket", "loon", "quanx", "v2rayn", "best",
    "static", "health", "task", "ss", "ssr", "vmess", "trojan", "vless", "sip002",
    "link", "debug", "bestProxyIp", "bestCfProxyIp", "bestCfProxyIpTop20",
    "bestCfProxyIpIsp", "bestCfProxyDomainTop20", "bestCfProxySub", "bestIpKr"];

// 获取部署前缀：服务端注入 > 前端从 location 自算（nginx 剥离前缀等场景兜底）
function getBasePath() {
    if (window.PROXYPOOL_BASE_PATH) return window.PROXYPOOL_BASE_PATH;
    var segs = location.pathname.split("/"); // ["", "show", "clash", ...]
    if (segs.length >= 3 && segs[1] && ROOT_ROUTES.indexOf(segs[1]) === -1) {
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
    // 主题初始化 + 跟随系统监听
    applyTheme(currentTheme());
    watchSystemTheme();
    var themeBtn = document.getElementById("theme-toggle");
    if (themeBtn) themeBtn.addEventListener("click", toggleTheme);

    // 移动端导航菜单切换（Tailwind 重构版：toggle hidden）
    var burger = document.getElementById("nav-burger");
    if (burger) {
        burger.addEventListener("click", function () {
            var menu = document.getElementById("nav-menu");
            if (menu) menu.classList.toggle("hidden");
        });
    }
    // 首页品牌链接跳转部署前缀首页
    var brand = document.getElementById("brand-home");
    if (brand) {
        brand.setAttribute("href", getBasePath() + "/");
    }
    // 渲染订阅链接：td 文本 / 复制按钮 / 一键导入
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
    // 动态生成器初始化：填充协议下拉并生成初始预览 URL
    if (document.getElementById("gen-client")) refreshBestGen();
    // 订阅表搜索过滤
    initSubSearch();
});

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
    if (mc) mc.setAttribute("content", dark ? "#020617" : "#f8fafc");
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

// 订阅表行复制（带按钮状态反馈：复制 → 已复制）
function copySubRow(btn) {
    var rel = btn.getAttribute("data-sub-path");
    if (rel && rel.indexOf("/best") === 0) ensureBestToken();
    copyText(getSubURL(rel)).then(function (ok) {
        if (ok) {
            var old = btn.innerHTML;
            btn.innerHTML = "✓ 已复制";
            setTimeout(function () { btn.innerHTML = old; }, 2000);
        }
        showTip(ok ? "复制成功" : "复制失败");
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
            rows[i].style.display = name.indexOf(kw) >= 0 ? "grid" : "none";
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
