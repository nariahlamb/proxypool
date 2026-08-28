// ProxyPool 前端脚本（原生 JS，无外部依赖）
// 功能：移动端导航切换、复制、任务触发（admin_token 鉴权）

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

// 复制按钮：从上一单元格取文本
function onCopy(e) {
    var cell = e.parentNode.previousElementSibling;
    copyText(cell ? cell.textContent.trim() : "").then(function (ok) {
        showTip(ok ? "复制成功" : "复制失败");
    });
}

// 复制按钮：从 data-copy 取链接
function onCopyThis(e) {
    copyText(e.getAttribute("data-copy") || "").then(function (ok) {
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
    var url = "/task/" + name + "?token=" + encodeURIComponent(token);
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
