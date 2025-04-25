// 显示历史记录
function loadHistory() {
    const historyList = document.getElementById("historyList");
    historyList.innerHTML = "";
    const items = JSON.parse(localStorage.getItem("clipHistory") || "[]");
    items.forEach(item => {
        const li = document.createElement("li");
        li.innerHTML = `<a href="${item.url}" target="_blank">${item.code}</a>`;
        historyList.appendChild(li);
    });
}

// 添加历史记录
function addToHistory(code) {
    const url = "/api/clip/get/" + code;
    const items = JSON.parse(localStorage.getItem("clipHistory") || "[]");
    items.unshift({ code, url });
    if (items.length > 10) items.pop(); // 最多保留10条
    localStorage.setItem("clipHistory", JSON.stringify(items));
    loadHistory();
}

// 表单提交逻辑
document.getElementById("createClipForm").addEventListener("submit", function (event) {
    event.preventDefault();

    fetch(this.action, {
        method: "POST",
        body: new FormData(this)
    })
        .then(response => response.json())
        .then(data => {
            if (data.code) {
                const codeUrl = "/api/clip/get/" + data.code;
                document.getElementById("result").innerHTML =
                    `✅ 创建成功：<a href="${codeUrl}" target="_blank">${data.code}</a>`;
                addToHistory(data.code);
            } else {
                document.getElementById("result").innerText = "❌ 错误: " + data.error;
            }
        })
        .catch(error => {
            document.getElementById("result").innerText = "❌ 错误: " + error;
        });
});

// 初始加载历史记录
loadHistory();