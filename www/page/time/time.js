const timeDisplay = document.getElementById('time');
const examInfoDisplay = document.getElementById('exam-info');
let currentExams = [];
let clockTimer = null;

async function getNtpTime() {
    try {
        const response = await fetch('/api/ntptime/');
        const data = await response.json();
        return new Date(data.timestamp);
    } catch (error) {
        console.error('获取 NTP 时间失败，使用本地时间', error);
        return new Date();
    }
}

function formatTime(ms) {
    const totalSeconds = Math.floor(ms / 1000);
    const hours = String(Math.floor(totalSeconds / 3600)).padStart(2, '0');
    const minutes = String(Math.floor((totalSeconds % 3600) / 60)).padStart(2, '0');
    const seconds = String(totalSeconds % 60).padStart(2, '0');
    return `${hours}:${minutes}:${seconds}`;
}

async function startClock(exams) {
    if (clockTimer) clearInterval(clockTimer);
    if (!exams || exams.length === 0) {
        examInfoDisplay.textContent = '';
        return;
    }

    currentExams = exams;
    let ntpTime = await getNtpTime();

    function update() {
        ntpTime = new Date(ntpTime.getTime() + 1000);
        timeDisplay.textContent = ntpTime.toLocaleTimeString('zh-CN', { hour12: false }).padStart(8, '0');

        const nextExam = currentExams.find(exam => {
            const end = new Date(new Date(exam.start_at).getTime() + exam.duration_hour * 3600000);
            return ntpTime < end;
        });

        if (!nextExam) {
            examInfoDisplay.textContent = '';
            return;
        }

        const start = new Date(nextExam.start_at).getTime();
        const end = start + nextExam.duration_hour * 3600000;

        let info = '';
        if (ntpTime.getTime() < start) {
            info = `${nextExam.name} | 未开始 | 剩余 ${formatTime(start - ntpTime.getTime())}`;
        } else if (ntpTime.getTime() < end) {
            info = `${nextExam.name} | 考试中 | 已过去 ${formatTime(ntpTime.getTime() - start)} | 剩余 ${formatTime(end - ntpTime.getTime())}`;
        }

        examInfoDisplay.innerHTML = info
            .split('|')
            .map((s, i, a) => i < a.length - 1
                ? `${s}<span class="sep">|</span>`
                : s)
            .join('');
    }

    update();
    clockTimer = setInterval(update, 1000);
}


async function handleExamClip() {
    const examCode = prompt("请输入 Exam Code：");
    if (!examCode) return;

    const url = `/api/clip/get/${encodeURIComponent(examCode)}`;

    try {
        const response = await fetch(url);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const exams = await response.json();

        document.getElementById('clip-result').textContent = `成功加载 ${exams.length} 条考试数据`;

        setInterval(() => {
            document.getElementById('clip-result').textContent = '';
        }, 3000);

        startClock(exams);
    } catch (err) {
        alert("加载考试数据失败：" + err.message);
    }
}

// 启动基础时间更新（即使没有考试数据也显示当前时间）
(async () => {
    let ntpTime = await getNtpTime();
    timeDisplay.textContent = ntpTime.toLocaleTimeString('zh-CN', { hour12: false }).padStart(8, '0');
    setInterval(async () => {
        ntpTime = new Date(ntpTime.getTime() + 1000);
        timeDisplay.textContent = ntpTime.toLocaleTimeString('zh-CN', { hour12: false }).padStart(8, '0');
    }, 1000);
})();