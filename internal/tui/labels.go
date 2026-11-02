package tui

import "github.com/beilunyang/openhongguo/sdk/model"

// Labels translate the service's enumerations, whose String() values are
// English identifiers.

func statusLabel(s model.SeriesStatus, lang string) string {
	zh := map[model.SeriesStatus]string{0: "更新中", 1: "已完结", 3: "今日更新", 4: "已停更"}
	en := map[model.SeriesStatus]string{0: "updating", 1: "finished", 3: "updated today", 4: "stopped"}
	if zh[s] == "" {
		return ""
	}
	return pick(lang, zh[s], en[s], s.String())
}

func payLabel(p model.PayType, lang string) string {
	zh := map[model.PayType]string{0: "免费", 1: "会员", 2: "单点付费"}
	en := map[model.PayType]string{0: "free", 1: "VIP", 2: "pay per title"}
	return pick(lang, zh[p], en[p], p.String())
}

func contentLabel(c model.ContentType, lang string) string {
	zh := map[model.ContentType]string{
		1: "短剧", 2: "情景剧", 3: "达人推荐", 4: "图文推荐", 5: "电影", 6: "电视剧", 7: "小短剧",
		1000: "素材", 1001: "PUGC", 1002: "AI 短剧", 1003: "合集", 1004: "漫剧",
	}
	en := map[model.ContentType]string{
		1: "short drama", 2: "scene play", 3: "creator pick", 4: "slideshow", 5: "movie", 6: "TV series", 7: "short play",
		1000: "material", 1001: "PUGC", 1002: "AI drama", 1003: "album", 1004: "motion comic",
	}
	if zh[c] == "" {
		return ""
	}
	return pick(lang, zh[c], en[c], c.String())
}

func platformLabel(p model.PlatformType, lang string) string {
	zh := map[model.PlatformType]string{0: "书/听书", 1: "抖音", 2: "UGC 视频", 3: "外部", 4: "西瓜", 5: "变色龙", 1024: "混合"}
	en := map[model.PlatformType]string{0: "book/audio", 1: "Douyin", 2: "UGC video", 3: "external", 4: "Xigua", 5: "Chameleon", 1024: "mixed"}
	return pick(lang, zh[p], en[p], p.String())
}

func infoKindLabel(k model.InfoKind, lang string) string {
	zh := map[string]string{
		"season": "季", "reason": "推荐语", "rank": "榜单", "category": "分类", "score": "评分",
		"author": "作者", "tags": "标签", "creation": "创作", "cast": "演员", "plot": "剧情", "other": "其他",
	}
	s := k.String()
	if lang == "zh" {
		if v, ok := zh[s]; ok {
			return v
		}
	}
	return s
}

func novelStatusLabel(s model.NovelStatus, lang string) string {
	zh := map[string]string{"finished": "已完结", "ongoing": "连载中", "updated today": "今日更新"}
	v := s.String()
	if lang == "zh" {
		if z, ok := zh[v]; ok {
			return z
		}
	}
	return v
}

// changeMark draws a rank change with ASCII so the column stays aligned in
// every terminal.
func changeMark(c *model.RankChange, lang string) string {
	if c == nil {
		return ""
	}
	switch c.Kind.String() {
	case "new":
		return pick(lang, "新", "new", "new")
	case "up":
		return "^" + itoa(c.Places)
	case "down":
		return "v" + itoa(c.Places)
	default:
		return "-"
	}
}

func pick(lang, zh, en, fallback string) string {
	if lang == "zh" && zh != "" {
		return zh
	}
	if en != "" {
		return en
	}
	return fallback
}

// discoverChannels are the channel names the service accepts.
var discoverChannels = []string{"推荐", "真人剧", "漫剧", "电影", "找剧", "经典", "知识", "大视听", "社区", "漫画", "小说", "听书"}

// searchTabs are the search result groups.
var searchTabs = []string{"综合", "剧集", "视频", "小说", "听书", "用户", "讨论"}

// searchFilters apply to the 综合 tab only.
var searchFilters = []string{"真人剧", "漫剧", "AI剧", "影视剧", "未看过"}

// filterGenres are the genres the short-drama filter panel knows.
var filterGenres = []string{"short_play", "comic_series", "ai_series"}

func genreLabel(g, lang string) string {
	zh := map[string]string{"short_play": "真人剧", "comic_series": "漫剧", "ai_series": "AI 剧"}
	en := map[string]string{"short_play": "live action", "comic_series": "motion comic", "ai_series": "AI drama"}
	return pick(lang, zh[g], en[g], g)
}
