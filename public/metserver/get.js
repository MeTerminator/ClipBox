const sleep = (delay) => new Promise((resolve) => setTimeout(resolve, delay)); // custom sleep func (only can use in async function with await)

function sliceText(text, maxLength) {
    // better slice()
    if (maxLength == 0) { // disabled
        return text;
    } else if (text.length <= maxLength) { // shorter than maxLength
        return text;
    }
    return text.slice(0, maxLength) + '...';
}

async function update() {
    let refresh_time = 2000;
    let routerIndex = window.location.href.indexOf('?');
    let url = window.location.href.slice(0, routerIndex > 0 ? routerIndex : window.location.href.length);
    while (true) {
        if (document.visibilityState == 'visible') {
            console.log('tab visible, updating...');
            let success_flag = true;
            let errorinfo = '';
            const statusElement = document.getElementById('status');
            // --- show updating
            // statusElement.textContent = '[更新中...]';
            // document.getElementById('additional-info').innerHTML = '正在更新状态...<br/>\n<a href="javascript:location.reload();" target="_self" style="color: rgb(0,149,255);">刷新页面</a>';
            last_status = statusElement.classList.item(0);
            statusElement.classList.remove(last_status);
            statusElement.classList.add('unknown');
            // fetch data
            const timestamp = Date.now();
            fetch(`/api/metserver/uptime/details/?t=${timestamp}`, { timeout: 10000 })
                .then(response => response.json())
                .then(async (data) => {
                    console.log(data);
                    // update status (status, additional-info)
                    last_status = statusElement.classList.item(0);
                    statusElement.classList.remove(last_status);
                    if (data.status == 'online') {
                        statusElement.textContent = '在线';
                        document.getElementById('additional-info').innerHTML = '在线时间: ' + data.last_stat_change + '<br/>\n平均延迟: ' + data.average_response_time +'ms';
                        statusElement.classList.add('alive');
                    } else if (data.status == 'offline') {
                        statusElement.textContent = '离线';
                        document.getElementById('additional-info').innerHTML = '离线时间: ' + data.last_stat_change + '<br/>\n平均延迟: ' + data.average_response_time +'ms';
                        statusElement.classList.add('error');
                    } else {
                        statusElement.textContent = '未知';
                        document.getElementById('additional-info').innerHTML = '状态保持时间: ' + data.last_stat_change + '<br/>\n平均延迟: ' + data.average_response_time +'ms';
                        statusElement.classList.add('unknown');
                    }
                    

                    // update last update time (last-updated)
                    const timestamp = data.last_updated * 1000;
                    const date = new Date(timestamp);
                    const formattedDate = date.toLocaleString();
                    document.getElementById('last-updated').textContent = formattedDate;

                })
                .catch(error => {
                    errorinfo = error;
                    success_flag = false;
                });
            // update error
            if (!success_flag) {
                statusElement.textContent = '[!错误!]';
                document.getElementById('additional-info').textContent = errorinfo;
                last_status = statusElement.classList.item(0);
                statusElement.classList.remove(last_status);
                statusElement.classList.add('error');
            }
        } else {
            console.log('tab not visible, skip update');
        }

        await sleep(refresh_time);
    }
}

update();



