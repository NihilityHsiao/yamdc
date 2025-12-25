package pornhub

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"go.uber.org/zap"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var defaultPornhubHosts = []string{
	//https://cn.pornhub.com/view_video.php?viewkey=680c20be0305a
	"https://cn.pornhub.com",
}

type pornhub struct {
	api.DefaultPlugin
}

func (p *pornhub) OnGetHosts(ctx context.Context) []string {
	return defaultPornhubHosts
}

func (p *pornhub) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	//番号格式
	if !strings.HasPrefix(number, "PORNHUB") {
		return false, nil
	}
	return true, nil
}
func (p *pornhub) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	num := strings.TrimPrefix(number, "PORNHUB-") //移除默认的前缀
	num = strings.ToLower(num)
	uri := fmt.Sprintf("%s/view_video.php?viewkey=%s", api.MustSelectDomain(defaultPornhubHosts), num)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *pornhub) decodeTitle(ctx context.Context) decoder.StringParseFunc {
	return func(v string) string {
		v = strings.TrimSuffix(v, " - Pornhub.com")
		v = strings.TrimSpace(v)
		return v
	}
}

func (p *pornhub) decodeGenre(ctx context.Context) decoder.StringListParseFunc {
	return func(gs []string) []string {

		for i, g := range gs {
			gs[i] = strings.TrimPrefix(g, "#")
			gs[i] = strings.TrimSpace(gs[i])
		}
		return gs
	}
}

func (p *pornhub) decodeDuration(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		// 21m30s
		// 01h16m56s
		return parser.HumanDurationToSecond(v)
	}
}
func (p *pornhub) decodeReleaseDate(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		logger := logutil.GetLogger(ctx).With(zap.String("releasedate", v))
		t, err := time.Parse("Jan 2, 2006", strings.TrimSpace(v))
		if err != nil {
			logger.Error("parse date time failed", zap.Error(err))
			return 0
		}
		return t.UnixMilli()
	}
}

func (p *pornhub) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {

	dec := decoder.XPathHtmlDecoder{
		//
		NumberExpr:    ``,
		TitleExpr:     `/html/head/title/text()`,
		PlotExpr:      ``,
		ActorListExpr: `//div[starts-with(@class, 'userInfo')]//div[contains(@class, 'usernameWrap')]//span[contains(@class, 'usernameBadgesWrapper')]//a//text()`,
		//ReleaseDateExpr:     `//span[starts-with(@class, 'VideoDetail_date')]/text()`,
		//DurationExpr:        `//div[contains(@class, 'VideoDetail_details')]//span[5]/text()`,
		StudioExpr:          ``,
		LabelExpr:           ``,
		DirectorExpr:        ``,
		SeriesExpr:          ``,
		GenreListExpr:       `//div[contains(@class, 'video-info-row')]//div[contains(@class,'categoriesWrapper')]//a[contains(@data-label, 'category')]`,
		CoverExpr:           `//meta[@property='og:image']/@content`, // 可能会报错https://cn.pornhub.com/view_video.php?viewkey=686ec6fc925e1
		PosterExpr:          `//meta[@property='og:image']/@content`,
		SampleImageListExpr: ``,
	}
	metadata, err := dec.DecodeHTML(data,
		decoder.WithTitleParser(p.decodeTitle(ctx)),
		decoder.WithGenreListParser(p.decodeGenre(ctx)),
		decoder.WithDurationParser(p.decodeDuration(ctx)),
		decoder.WithReleaseDateParser(p.decodeReleaseDate(ctx)))
	if err != nil {
		return nil, false, err
	}

	// xpath中的日期是 xx个月前, xx年前
	// 真实的日期是在html的script脚本中, 关键词为 video_date_published
	re := regexp.MustCompile(`window\.dataLayer\.push\(\s*(\{[\s\S]*?})\s*\);`)
	matches := re.FindSubmatch(data)

	if len(matches) >= 2 {
		var videoData pornhubVideoData
		// 主要是获取duration和releaseDate, 失败了也没事
		jsonStr := strings.ReplaceAll(string(matches[1]), "'", "\"")
		jsonErr := json.Unmarshal([]byte(jsonStr), &videoData)
		if jsonErr == nil {
			metadata.Duration = p.readDuration(&videoData)
			metadata.ReleaseDate = p.readReleaseDate(&videoData)
		}
	}

	metadata.Number = meta.GetNumberId(ctx)
	metadata.TitleLang = enum.MetaLangEn
	return metadata, true, nil
}

func (p *pornhub) readDuration(data *pornhubVideoData) int64 {
	minutes, err := strconv.ParseInt(data.VideoData.VideoDuration, 10, 64)
	if err != nil {
		return 0
	}
	return minutes * 60
}

func (p *pornhub) readReleaseDate(data *pornhubVideoData) int64 {
	t, err := time.Parse("20060102", strings.TrimSpace(data.VideoData.VideoDatePublished))
	if err != nil {
		return 0
	}

	return t.UnixMilli()
}

func init() {
	factory.Register(constant.SSPornHub, factory.PluginToCreator(&pornhub{}))

}
