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
    // best 优选订阅鉴权：复制/渲染 /best* 链接时自动拼 best_token（localStorage 输入一次）
    if (rel.indexOf("/best") === 0) {
        var token = localStorage.getItem("proxypool_best_token") || "";
        if (!token) {
            token = window.prompt("请输入 best_token（config.yaml 中配置，用于优选订阅鉴权）：");
            if (token) localStorage.setItem("proxypool_best_token", token);
        }
        if (token) url += (url.indexOf("?") >= 0 ? "&" : "?") + "token=" + encodeURIComponent(token);
    }
    return url;
}

document.addEventListener("DOMContentLoaded", function () {
    // 移动端导航菜单切换
    var burger = document.querySelector(".navbar-burger");
    if (burger) {
        burger.addEventListener("click", function () {
            burger.classList.toggle("is-active");
            var menu = document.getElementById(burger.getAttribute("data-target"));
            if (menu) menu.classList.toggle("is-active");
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
});

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

function showTip(msg) {
    var el = document.createElement("div");
    el.className = "notification has-text-primary";
    el.innerHTML = "<i>✔</i><p>" + msg + "</p>";
    document.body.appendChild(el);
    setTimeout(function () { el.classList.add("show"); }, 30);
    setTimeout(function () {
        el.classList.remove("show");
        setTimeout(function () { el.remove(); }, 500);
    }, 1800);
}

// 复制按钮：从上一单元格（td[data-sub-path]）取相对路径拼接完整 URL
function onCopy(e) {
    var cell = e.parentNode.previousElementSibling;
    var rel = cell ? cell.getAttribute("data-sub-path") : "";
    copyText(getSubURL(rel)).then(function (ok) {
        showTip(ok ? "复制成功" : "复制失败");
    });
}

// 复制按钮：从 data-sub-path 拼接完整 URL
function onCopyThis(e) {
    copyText(getSubURL(e.getAttribute("data-sub-path"))).then(function (ok) {
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
