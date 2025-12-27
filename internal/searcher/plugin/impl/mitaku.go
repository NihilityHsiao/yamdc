package impl

import (
	"context"
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
	"strings"
	"time"
)

var defaultMitakuHosts = []string{
	"https://mitaku.net",
}

type mitaku struct {
	api.DefaultPlugin
}

func (p *mitaku) OnGetHosts(ctx context.Context) []string {
	return defaultMitakuHosts
}

func (p *mitaku) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	if !strings.HasPrefix(number, "MITAKU") {
		return false, nil
	}
	return true, nil
}
func (p *mitaku) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	num := strings.TrimPrefix(number, "MITAKU-") //移除默认的前缀
	uri := fmt.Sprintf("%s/ero-cosplay/%s", api.MustSelectDomain(defaultMitakuHosts), num)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *mitaku) decodeTitle(ctx context.Context) decoder.StringParseFunc {
	return func(v string) string {
		// MollyRedWolf - 标题 - ManyVids
		// 去除最后的 ManyVids

		n := strings.SplitN(v, "–", 2)
		if len(n) > 1 {
			v = strings.TrimSpace(n[1])
		}
		return v
	}
}

func (p *mitaku) decodeGenre(ctx context.Context) decoder.StringListParseFunc {

	genresKeys := []string{
		"Cosplayer:",
		"Character:",
		"Game:",
	}

	return func(gs []string) []string {

		ret := make([]string, 0)

		for _, k := range genresKeys {
			for _, g := range gs {
				if strings.Contains(g, k) {
					genre := strings.ReplaceAll(g, k, "")
					ret = append(ret, strings.TrimSpace(genre))
				}
			}
		}

		return ret
	}
}

func (p *mitaku) decodeDuration(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		// 21m30s
		// 01h16m56s
		return parser.HumanDurationToSecond(v)
	}
}
func (p *mitaku) decodeReleaseDate(ctx context.Context) decoder.NumberParseFunc {
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

func (p *mitaku) decodeActorList(ctx context.Context) decoder.StringListParseFunc {
	return func(v []string) []string {
		if len(v) == 0 {
			return nil
		}
		n := strings.SplitN(v[0], "–", 2)
		if len(n) > 1 {
			v[0] = strings.TrimSpace(n[0])
		}
		return v
	}
}

func (p *mitaku) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr: ``,
		//TitleExpr:           `/html/head/title/text()`,
		TitleExpr:           `//h1[starts-with(@class, 'entry-title')]/text()`,
		PlotExpr:            ``,
		ActorListExpr:       `//h1[starts-with(@class, 'entry-title')]/text()`,
		ReleaseDateExpr:     ``,
		DurationExpr:        `//div[contains(@class, 'VideoDetail_details')]//span[5]/text()`,
		StudioExpr:          ``,
		LabelExpr:           ``,
		DirectorExpr:        ``,
		SeriesExpr:          ``,
		GenreListExpr:       `//div[starts-with(@class, 'entry-content')]/p/text()`,
		CoverExpr:           `//meta[@property='og:image']/@content`,
		PosterExpr:          `//meta[@property='og:image']/@content`,
		SampleImageListExpr: `//a[@class='msacwl-img-link']/@data-mfp-src`,
	}
	metadata, err := dec.DecodeHTML(data,
		decoder.WithTitleParser(p.decodeTitle(ctx)),
		decoder.WithGenreListParser(p.decodeGenre(ctx)),
		decoder.WithActorListParser(p.decodeActorList(ctx)),
		decoder.WithDurationParser(p.decodeDuration(ctx)),
		decoder.WithReleaseDateParser(p.decodeReleaseDate(ctx)))
	if err != nil {
		return nil, false, err
	}

	metadata.Number = meta.GetNumberId(ctx)
	metadata.TitleLang = enum.MetaLangZH
	metadata.SwithConfig.DisableReleaseDateCheck = true

	return metadata, true, nil
}

func init() {
	factory.Register(constant.SSMitaku, factory.PluginToCreator(&mitaku{}))
}
