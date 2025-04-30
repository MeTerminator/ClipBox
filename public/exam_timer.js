const timeDisplay = document.getElementById('time');
const examInfoDisplay = document.getElementById('exam-info');

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

async function loadExams() {
  try {
    const response = await fetch('/api/exams/');
    return await response.json();
  } catch (error) {
    console.error('加载考试数据失败', error);
    return [];
  }
}

function formatTime(ms) {
  const totalSeconds = Math.floor(ms / 1000);
  const hours = String(Math.floor(totalSeconds / 3600)).padStart(2, '0');
  const minutes = String(Math.floor((totalSeconds % 3600) / 60)).padStart(2, '0');
  const seconds = String(totalSeconds % 60).padStart(2, '0');
  return `${hours}:${minutes}:${seconds}`;
}

async function startClock() {
  const exams = await loadExams();
  let ntpTime = await getNtpTime();

  function update() {
    ntpTime = new Date(ntpTime.getTime() + 1000);
    timeDisplay.textContent = ntpTime.toLocaleTimeString('zh-CN', { hour12: false }).padStart(8, '0');

    const nextExam = exams.find(exam => {
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
        ? `${s}<span class="text-gray-500 mx-2">|</span>`
        : s)
      .join('');
  }

  update();
  setInterval(update, 1000);
}

startClock();