const codeEl = document.getElementById("pickupCode");
const btnEl = document.getElementById("pickupBtn");
const go = () => {
  const code = (codeEl.value || "").trim();
  if (!/^\d{5}$/.test(code)) {
    codeEl.focus();
    codeEl.select();
    alert("请输入有效的 5 位取件码");
    return;
  }
  window.location.href = `/api/clip/get/${code}`;
};
btnEl.addEventListener("click", go);
codeEl.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    go();
  }
});
document.getElementById("year").textContent = new Date().getFullYear();
