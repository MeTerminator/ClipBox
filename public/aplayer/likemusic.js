function aplayerInit(fixed = false, mini = false) {
    axios.get('/api/likemusic/')
        .then(response => {
            const rawList = response.data;
            // 构造 APlayer 所需的 audios 列表
            const audios = rawList.map(item => ({
                name: item.name || '未知标题',
                artist: item.artist || '未知歌手',
                url: item.url,
                cover: item.cover || '',  // 可选封面
                lrc: `https://api.injahow.cn/meting/?server=tencent&type=lrc&id=${item.mid}`
            }));

            // 初始化 APlayer
            window.aplayer = new APlayer({
                container: document.getElementById('aplayer'),
                fixed: fixed,  // 固定在顶部
                mini: mini,  // 迷你模式
                lrcType: 3, // 启用歌词
                audio: audios
            });
        })
        .catch(error => {
            console.error('获取歌曲列表失败：', error);
        });
}