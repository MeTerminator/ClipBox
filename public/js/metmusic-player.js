// metmusic-player.js

class MeTMusicPlayer {
    /**
     * @param {HTMLAudioElement} audioElement - 需要绑定的 HTMLAudioElement 实例。
     * @param {string} sessionId - 用于 WebSocket 连接的会话 ID。
     * @param {object} [options] - 可选配置项。
     * @param {object} [options.domElements] - 可选的DOM元素映射。
     * @param {HTMLElement} [options.domElements.statusText] - 状态文本元素。
     * @param {HTMLElement} [options.domElements.songnameText] - 歌曲名称文本元素。
     * @param {HTMLElement} [options.domElements.singerText] - 歌手文本元素。
     * @param {HTMLElement} [options.domElements.albumText] - 专辑文本元素。
     * @param {HTMLElement} [options.domElements.midText] - 歌曲MID文本元素。
     * @param {HTMLElement} [options.domElements.startTimeText] - 开始时间文本元素。
     * @param {HTMLProgressElement} [options.domElements.progressBar] - 进度条元素。
     * @param {HTMLElement} [options.domElements.currentTimeText] - 当前时间文本元素。
     * @param {HTMLElement} [options.domElements.durationText] - 总时长文本元素。
     * @param {HTMLInputElement} [options.domElements.volumeSlider] - 音量滑块元素。
     * @param {Function} [options.onChange] - 状态变化时的回调函数，传入当前播放器实例。
     */
    constructor(audioElement, sessionId, options = {}) {
        if (!audioElement) {
            console.error("需要提供一个有效的 <audio> 元素。");
            return;
        }

        this.audioPlayer = audioElement;
        this.SID = sessionId;
        this.options = options;

        // 绑定DOM元素
        this.statusText = options.domElements?.statusText || null;
        this.songnameText = options.domElements?.songnameText || null;
        this.singerText = options.domElements?.singerText || null;
        this.albumText = options.domElements?.albumText || null;
        this.midText = options.domElements?.midText || null;
        this.startTimeText = options.domElements?.startTimeText || null;
        this.progressBar = options.domElements?.progressBar || null;
        this.currentTimeText = options.domElements?.currentTimeText || null;
        this.durationText = options.domElements?.durationText || null;
        this.volumeSlider = options.domElements?.volumeSlider || null;
        
        // 新增：on_change 回调
        this.onChangeCallback = options.onChange || null;

        // 如果传入了进度条元素，则禁止用户交互
        if (this.progressBar) {
            this.progressBar.disabled = true;
        }

        // 状态变量
        this.ws = null;
        this.currentServerStartTime = 0;
        this.midUrlCache = {};
        this.isSeekPending = false; 
        this.musicStatus = false;
        this.musicMid = "";
        this.lastUpdateTs = 0;
        this.lastMusicMid = "";
        this.lastMusicStartTs = 0;
        this.syncInterval = null;
        this.songData = null;

        // 加载时间优化相关变量
        this.loadingTimes = [];
        this.loadingStartTime = 0;
        this.averageLoadingTime = 2000; // 初始默认值 2 秒
        this.minPreloadTime = 500; // 最短预加载时间 0.5 秒

        this._init();
    }

    _init() {
        this._bindEvents();
        this.connectWebSocket();
    }

    // 触发 on_change 回调
    _triggerOnChange() {
        if (this.onChangeCallback && typeof this.onChangeCallback === 'function') {
            this.onChangeCallback(this);
        }
    }

    _updateUI(element, content) {
        if (element) {
            element.textContent = content;
        }
    }

    _formatTime(seconds) {
        if (isNaN(seconds) || seconds < 0) return "0:00";
        const min = Math.floor(seconds / 60);
        const sec = Math.floor(seconds % 60);
        return `${min}:${sec < 10 ? '0' : ''}${sec}`;
    }

    _bindEvents() {
        this.audioPlayer.addEventListener('canplay', () => {
            const loadingTime = Date.now() - this.loadingStartTime;
            if (loadingTime > 0) {
                this._addLoadingTime(loadingTime);
            }
            if (this.isSeekPending) {
                const accurateSeekTime = Math.max(0, (Date.now() - this.currentServerStartTime) / 1000);
                this.audioPlayer.currentTime = accurateSeekTime;
                this.isSeekPending = false;
            }
            this.audioPlayer.play().catch(error => {
                console.error("播放被浏览器阻止:", error);
                this._updateUI(this.statusText, '播放失败，请进行用户交互。');
            });
            this._triggerOnChange(); // 歌曲加载完毕，触发回调
        });

        this.audioPlayer.addEventListener('play', () => { 
            this._updateUI(this.statusText, '播放中');
            this._triggerOnChange(); // 播放状态变化，触发回调
        });
        this.audioPlayer.addEventListener('pause', () => { 
            if (this.audioPlayer.src && !this.audioPlayer.ended) {
                this._updateUI(this.statusText, '已暂停'); 
            }
            this._triggerOnChange(); // 播放状态变化，触发回调
        });
        this.audioPlayer.addEventListener('ended', () => { 
            this._updateUI(this.statusText, '播放结束');
            this._triggerOnChange(); // 播放结束，触发回调
        });
        
        this.audioPlayer.addEventListener('timeupdate', () => {
            if (!isNaN(this.audioPlayer.currentTime) && !isNaN(this.audioPlayer.duration)) {
                if (this.progressBar) this.progressBar.value = this.audioPlayer.currentTime;
                this._updateUI(this.currentTimeText, this._formatTime(this.audioPlayer.currentTime));
            }
            this._triggerOnChange(); // 进度变化，触发回调
        });

        this.audioPlayer.addEventListener('durationchange', () => {
            if (!isNaN(this.audioPlayer.duration)) {
                if (this.progressBar) this.progressBar.max = this.audioPlayer.duration;
                this._updateUI(this.durationText, this._formatTime(this.audioPlayer.duration));
            }
            this._triggerOnChange(); // 总时长变化，触发回调
        });

        if (this.volumeSlider) {
            this.volumeSlider.addEventListener('input', (e) => {
                this.audioPlayer.volume = e.target.value;
                if (e.target.value > 0) { this.audioPlayer.muted = false; }
            });
            this.audioPlayer.volume = this.volumeSlider.value;
        }
    }

    // --- 公共 API ---

    getPlayerStatus() {
        return {
            isPlaying: this.musicStatus,
            songMid: this.musicMid,
            startTime: this.currentServerStartTime,
            songDetails: this.songData,
            currentTime: this.audioPlayer.currentTime,
            duration: this.audioPlayer.duration,
            volume: this.audioPlayer.volume
        };
    }

    // --- 加载时间优化方法 ---

    _addLoadingTime(timeInMs) {
        this.loadingTimes.push(timeInMs);
        if (this.loadingTimes.length > 5) {
            this.loadingTimes.shift();
        }
        const sum = this.loadingTimes.reduce((a, b) => a + b, 0);
        this.averageLoadingTime = sum / this.loadingTimes.length;
        console.log(`平均加载时间: ${this.averageLoadingTime.toFixed(2)}ms`);
    }

    // --- 核心逻辑方法 ---

    connectWebSocket() {
        this._updateUI(this.statusText, '正在连接服务器...');
        const wsUrl = "wss://music.met6.top:444/api-client/ws/listen";
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            this._updateUI(this.statusText, '服务器已连接');
            this.ws.send(JSON.stringify({ type: "listen", SessionId: [this.SID] }));
            setInterval(() => this._statusCheck(), 1000);
        };

        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                if (data.type === "feedback" && data.SessionId === this.SID) {
                    this.musicStatus = data.data.status;
                    this.musicMid = data.data.songMid;
                    const musicStartTs = data.data.systemTime - data.data.currentTime * 1000;
                    this.lastUpdateTs = Date.now();
                    
                    this._updateMusicStatus(this.musicStatus, this.musicMid, musicStartTs);
                }
            } catch (e) {
                console.error("解析消息失败:", e);
            }
        };

        this.ws.onclose = () => {
            this._updateUI(this.statusText, '连接已断开，5秒后重试...');
            clearInterval(this.syncInterval);
            this.syncInterval = null;
            setTimeout(() => this.connectWebSocket(), 5000);
        };

        this.ws.onerror = (error) => {
            this._updateUI(this.statusText, '连接错误');
            console.error("WebSocket Error:", error);
        };
    }

    _updateMusicStatus(currentStatus, currentMid, currentStartTs) {
        if (currentStatus) {
            const isNewSong = currentMid !== this.lastMusicMid;
            const isTimeDrifted = Math.abs(currentStartTs - this.lastMusicStartTs) > 500;

            if (isNewSong || isTimeDrifted) {
                const shouldPlayTime = currentStartTs;
                const preloadTime = Math.max(this.averageLoadingTime, this.minPreloadTime);
                const preloadStartTime = shouldPlayTime - preloadTime;

                const now = Date.now();
                if (now < preloadStartTime) {
                    const delay = preloadStartTime - now;
                    this._updateUI(this.statusText, `将在 ${delay.toFixed(0)}ms 后预加载...`);
                    console.log(`[预加载] 延迟 ${delay.toFixed(0)}ms, 预加载时长 ${preloadTime.toFixed(0)}ms`);
                    setTimeout(() => {
                        this._prepareToPlay(currentMid, currentStartTs);
                    }, delay);
                } else {
                    this._prepareToPlay(currentMid, currentStartTs);
                }
            }

            this.lastMusicStartTs = currentStartTs;
            this.lastMusicMid = currentMid;
        } else {
            this._handlePause();
        }
    }
    
    _statusCheck() {
        if (this.musicStatus && (Date.now() - this.lastUpdateTs > 12000)) {
            this.musicStatus = false;
            this._updateMusicStatus(false, "", 0);
        }
    }

    _updateSongInfo(trackInfo) {
        if (trackInfo) {
            const songName = trackInfo.title || '未知歌曲';
            const singers = trackInfo.singer?.map(s => s.name).join(', ') || '未知歌手';
            const albumName = trackInfo.album?.name || '未知专辑';

            this._updateUI(this.songnameText, songName);
            this._updateUI(this.singerText, singers);
            this._updateUI(this.albumText, albumName);
        } else {
            this._updateUI(this.songnameText, '-');
            this._updateUI(this.singerText, '-');
            this._updateUI(this.albumText, '-');
            this._updateUI(this.midText, '-');
            this._updateUI(this.startTimeText, '-');
        }
        this._triggerOnChange(); // 歌曲信息变化，触发回调
    }

    async _getSongUrl(mid) {
        if (this.midUrlCache[mid]) return this.midUrlCache[mid];
        try {
            const response = await fetch(`https://music.met6.top:444/api/song/url/v1/?id=${mid}&level=hq`);
            const data = await response.json();
            this.songData = data;
            const url = data.data[0]?.url;
            if (url) {
                this.midUrlCache[mid] = { 
                    url: url,
                    track_info: data.data[0]?.track_info
                };
                return url;
            }
            return "";
        } catch (e) {
            console.error("获取歌曲链接失败:", e);
            return "";
        }
    }
    
    async _prepareToPlay(mid, startTime) {
        this._updateUI(this.statusText, '获取歌曲信息...');
        const cachedData = this.midUrlCache[mid];
        
        let songUrl;
        let trackInfo;

        if (cachedData) {
            songUrl = cachedData.url;
            trackInfo = cachedData.track_info;
        } else {
            const fetchedUrl = await this._getSongUrl(mid);
            songUrl = fetchedUrl;
            trackInfo = this.midUrlCache[mid]?.track_info;
        }
        
        if (songUrl) {
            this._updateUI(this.statusText, '正在缓冲音频...');
            this.currentServerStartTime = startTime;
            this._updateUI(this.midText, mid);
            this._updateUI(this.startTimeText, new Date(startTime).toLocaleString('zh-CN'));
            this._updateSongInfo(trackInfo);
            
            this.isSeekPending = true;
            this.audioPlayer.src = songUrl;

            this.loadingStartTime = Date.now();
            
            clearInterval(this.syncInterval);
            this.syncInterval = setInterval(() => this._checkAndSync(), 1000);
        } else {
            this._updateUI(this.statusText, '获取歌曲链接失败');
            this._updateSongInfo(null);
        }
    }

    _handlePause() {
        this.audioPlayer.pause();
        this._updateUI(this.statusText, '已暂停');
        this._updateUI(this.midText, '-');
        this._updateUI(this.startTimeText, '-');
        if (this.progressBar) this.progressBar.value = 0;
        this._updateUI(this.currentTimeText, '0:00');
        
        clearInterval(this.syncInterval);
        this.syncInterval = null;
        this._triggerOnChange(); // 暂停状态变化，触发回调
    }
    
    _checkAndSync() {
        if (!this.musicStatus || !this.audioPlayer.src || this.audioPlayer.paused || this.audioPlayer.ended) {
            return;
        }
        
        const expectedTime = (Date.now() - this.currentServerStartTime) / 1000;
        const actualTime = this.audioPlayer.currentTime;
        const timeDiff = Math.abs(expectedTime - actualTime);
        
        if (timeDiff > 0.5) {
            console.warn(`检测到时间偏差：${timeDiff.toFixed(2)}s。正在重新同步。`);
            this.audioPlayer.currentTime = expectedTime;
        }
        this._triggerOnChange(); // 进度变化，触发回调
    }
}
