package tui

// Strings are the words of the interface in one language. Content from the
// service (titles, tags) is never translated.
type Strings struct {
	AppTitle string

	SecContinue, SecDiscover, SecHot, SecRank, SecFilter, SecNew string
	SecHistory, SecDownloads, SecBooks, SecSettings              string

	Loading, Empty, EmptyContinue, EmptyHistory, EmptyDownloads string
	Episodes, EpisodeN, Playing, Idle, LastWatched, Finished    string
	Heat, Plays, Follows, Likes, Status, Pay, Kind, Platform    string
	Released, Badge, Novel, Seasons, Related, Description       string
	Author, Words, Category, Followers, Works, Online, Upcoming string
	Guesses, Topics, Promoted, Date, Tab, Results, Cursor       string

	KeyMove, KeyOpen, KeyBack, KeyPlay, KeyAutoplay, KeyResume  string
	KeyDownload, KeyTabs, KeyFocus, KeySearch, KeyHelp, KeyQuit string
	KeySection, KeyLang, KeyToggle, KeyClear, KeyApply, KeyStop string
	KeyPage, KeyTopBottom, KeyNav, KeyChange, KeyRemove         string
	KeyContent, KeyBoard, KeyPerson, KeyCancel, KeyRefresh      string

	HelpTitle, HelpGlobal, HelpScreen string

	SetPlayer, SetQuality, SetAutoplay, SetDownloadDir, SetJobs     string
	SetCovers, SetTheme, SetLanguage, SetResetDevice, SetDefaultDir string
	On, Off                                                         string

	ToastSaved, ToastDeviceReset, ToastPlaying, ToastPlaybackDone string
	ToastAlreadyPlaying, ToastBook, ToastLocked, ToastStopped     string
	ToastSkipped, ToastHistory, ToastQueued, ToastDownloadDone    string
	ToastDownloadFailed, ToastCleared, ToastNotYet                string

	ErrAnonymous, ErrAuth, ErrDRM, ErrGone, ErrRate, ErrNetwork string
	ErrDevice, ErrEnd, ErrOther                                 string

	PromptEpisodes, PromptCategory, PromptConfirmClear, SearchPlaceholder string
	LegendWatched, LegendCurrent, LegendLocked, LegendNew                 string
	Yes, No, Done, Failed, Waiting, Saving, Skipped                       string
	Language, Theme                                                       string
}

var zhStrings = Strings{
	AppTitle: "红果短剧",

	SecContinue: "继续观看", SecDiscover: "推荐", SecHot: "热搜", SecRank: "排行榜", SecFilter: "筛选", SecNew: "上新",
	SecHistory: "观看记录", SecDownloads: "下载", SecBooks: "书城", SecSettings: "设置",

	Loading: "加载中…", Empty: "没有内容", EmptyContinue: "还没有看过短剧。去「推荐」或按 / 搜索。",
	EmptyHistory: "没有观看记录", EmptyDownloads: "下载队列为空。在剧集页按 d 下载。",
	Episodes: "%d 集", EpisodeN: "第 %d 集", Playing: "正在播放", Idle: "空闲", LastWatched: "上次看到", Finished: "已看完",
	Heat: "热度", Plays: "播放", Follows: "追剧", Likes: "点赞", Status: "状态", Pay: "付费", Kind: "类型", Platform: "来源",
	Released: "上线", Badge: "角标", Novel: "原著小说", Seasons: "季", Related: "相关推荐", Description: "简介",
	Author: "作者", Words: "字数", Category: "分类", Followers: "粉丝", Works: "作品", Online: "已上线", Upcoming: "即将上线",
	Guesses: "猜你想搜", Topics: "热点话题", Promoted: "推广", Date: "日期", Tab: "栏目", Results: "结果", Cursor: "下一页",

	KeyMove: "移动", KeyOpen: "打开", KeyBack: "返回", KeyPlay: "播放", KeyAutoplay: "从这集连播", KeyResume: "续播",
	KeyDownload: "下载", KeyTabs: "切换标签", KeyFocus: "切换焦点", KeySearch: "搜索", KeyHelp: "帮助", KeyQuit: "退出",
	KeySection: "跳到栏目", KeyLang: "切换语言", KeyToggle: "选中/取消", KeyClear: "清空", KeyApply: "应用", KeyStop: "停止播放",
	KeyPage: "翻页", KeyTopBottom: "顶部/底部", KeyNav: "栏目", KeyChange: "修改", KeyRemove: "移除",
	KeyContent: "内容类型", KeyBoard: "榜单", KeyPerson: "演员主页", KeyCancel: "取消", KeyRefresh: "刷新",

	HelpTitle: "快捷键", HelpGlobal: "全局", HelpScreen: "当前页面",

	SetPlayer: "播放器", SetQuality: "画质", SetAutoplay: "连续播放", SetDownloadDir: "下载目录", SetJobs: "并发下载",
	SetCovers: "显示封面", SetTheme: "主题", SetLanguage: "语言", SetResetDevice: "重置匿名设备", SetDefaultDir: "默认（以剧名命名）",
	On: "开", Off: "关",

	ToastSaved: "设置已保存", ToastDeviceReset: "已注册新的匿名设备", ToastPlaying: "播放 %s · 第 %d 集", ToastPlaybackDone: "播放结束",
	ToastAlreadyPlaying: "已经有正在播放的内容，先按 S 停止", ToastBook: "这是书或听书，不能播放", ToastLocked: "这一集匿名设备不能播放",
	ToastStopped: "已停止播放", ToastSkipped: "第 %d 集不可用，跳过", ToastHistory: "观看记录没有保存：%v", ToastQueued: "已加入下载队列：%s",
	ToastDownloadDone: "下载完成：%s", ToastDownloadFailed: "%d 集下载失败", ToastCleared: "已清空", ToastNotYet: "这个功能还没有做",

	ErrAnonymous: "匿名设备拿不到这个数据", ErrAuth: "需要登录账号", ErrDRM: "加密分集", ErrGone: "内容不存在或已下架",
	ErrRate: "请求太频繁，稍后再试", ErrNetwork: "网络错误", ErrDevice: "设备被拒绝，正在重新注册", ErrEnd: "没有更多了", ErrOther: "出错了",

	PromptEpisodes: "下载哪些集？例如 1-10,12，留空下载全部", PromptCategory: "分类 ID（例如 68）", PromptConfirmClear: "清空观看记录？输入 yes 确认",
	SearchPlaceholder: "搜索短剧、演员、小说…",
	LegendWatched:     "已看", LegendCurrent: "当前", LegendLocked: "锁定", LegendNew: "新",
	Yes: "是", No: "否", Done: "完成", Failed: "失败", Waiting: "等待", Saving: "下载中", Skipped: "已存在",
	Language: "中文", Theme: "主题",
}

var enStrings = Strings{
	AppTitle: "Hongguo",

	SecContinue: "Continue", SecDiscover: "Discover", SecHot: "Trending", SecRank: "Rankings", SecFilter: "Filter", SecNew: "New",
	SecHistory: "History", SecDownloads: "Downloads", SecBooks: "Books", SecSettings: "Settings",

	Loading: "Loading…", Empty: "Nothing here", EmptyContinue: "Nothing watched yet. Try Discover, or press / to search.",
	EmptyHistory: "No watch history", EmptyDownloads: "The download queue is empty. Press d on a series page.",
	Episodes: "%d eps", EpisodeN: "Ep %d", Playing: "Playing", Idle: "Idle", LastWatched: "Last watched", Finished: "Finished",
	Heat: "Heat", Plays: "Plays", Follows: "Follows", Likes: "Likes", Status: "Status", Pay: "Pay", Kind: "Type", Platform: "Source",
	Released: "Released", Badge: "Badge", Novel: "Novel", Seasons: "Seasons", Related: "Related", Description: "Synopsis",
	Author: "Author", Words: "Words", Category: "Category", Followers: "Followers", Works: "Works", Online: "Online", Upcoming: "Upcoming",
	Guesses: "You might search", Topics: "Topics", Promoted: "promoted", Date: "Date", Tab: "Tab", Results: "Results", Cursor: "Next page",

	KeyMove: "move", KeyOpen: "open", KeyBack: "back", KeyPlay: "play", KeyAutoplay: "autoplay from here", KeyResume: "resume",
	KeyDownload: "download", KeyTabs: "switch tab", KeyFocus: "switch focus", KeySearch: "search", KeyHelp: "help", KeyQuit: "quit",
	KeySection: "jump to section", KeyLang: "toggle language", KeyToggle: "toggle", KeyClear: "clear", KeyApply: "apply", KeyStop: "stop playback",
	KeyPage: "page", KeyTopBottom: "top/bottom", KeyNav: "sections", KeyChange: "change", KeyRemove: "remove",
	KeyContent: "content type", KeyBoard: "board", KeyPerson: "person page", KeyCancel: "cancel", KeyRefresh: "refresh",

	HelpTitle: "Keys", HelpGlobal: "Everywhere", HelpScreen: "This screen",

	SetPlayer: "Player", SetQuality: "Quality", SetAutoplay: "Autoplay", SetDownloadDir: "Download folder", SetJobs: "Parallel downloads",
	SetCovers: "Show covers", SetTheme: "Theme", SetLanguage: "Language", SetResetDevice: "Reset anonymous device", SetDefaultDir: "default (series title)",
	On: "on", Off: "off",

	ToastSaved: "Settings saved", ToastDeviceReset: "New anonymous device registered", ToastPlaying: "Playing %s · ep %d", ToastPlaybackDone: "Playback ended",
	ToastAlreadyPlaying: "Something is already playing; press S to stop it", ToastBook: "This is a book or audiobook; it cannot be played", ToastLocked: "This episode cannot be played anonymously",
	ToastStopped: "Playback stopped", ToastSkipped: "Episode %d unavailable; skipped", ToastHistory: "History not saved: %v", ToastQueued: "Queued for download: %s",
	ToastDownloadDone: "Download finished: %s", ToastDownloadFailed: "%d episodes failed", ToastCleared: "Cleared", ToastNotYet: "Not built yet",

	ErrAnonymous: "Not available to an anonymous device", ErrAuth: "Needs an account", ErrDRM: "Encrypted episode", ErrGone: "Content missing or taken down",
	ErrRate: "Too many requests; try again later", ErrNetwork: "Network error", ErrDevice: "Device rejected; registering a new one", ErrEnd: "No more", ErrOther: "Something went wrong",

	PromptEpisodes: "Which episodes? e.g. 1-10,12; empty for all", PromptCategory: "Category ID (e.g. 68)", PromptConfirmClear: "Clear watch history? Type yes to confirm",
	SearchPlaceholder: "Search dramas, people, novels…",
	LegendWatched:     "watched", LegendCurrent: "current", LegendLocked: "locked", LegendNew: "new",
	Yes: "yes", No: "no", Done: "done", Failed: "failed", Waiting: "waiting", Saving: "saving", Skipped: "exists",
	Language: "English", Theme: "Theme",
}

func stringsFor(lang string) *Strings {
	if lang == "en" {
		return &enStrings
	}
	return &zhStrings
}
