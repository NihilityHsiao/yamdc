package searcher

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/hasher"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/store"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

// URLSearcher 直接从指定URL获取数据并使用指定plugin解析
type URLSearcher struct {
	name      string
	targetURL string
	cc        *config
	plg       api.IPlugin
}

// NewURLSearcher 创建一个URL Searcher
// name: searcher名称
// targetURL: 要请求的目标URL
// plg: 用于解析的plugin
// opts: 可选配置
func NewURLSearcher(name string, targetURL string, plg api.IPlugin, opts ...Option) (ISearcher, error) {
	if targetURL == "" {
		return nil, fmt.Errorf("target URL cannot be empty")
	}
	cc := applyOpts(opts...)
	return &URLSearcher{
		name:      name,
		targetURL: targetURL,
		cc:        cc,
		plg:       plg,
	}, nil
}

func (s *URLSearcher) Name() string {
	return s.name
}

func (s *URLSearcher) Check(ctx context.Context) error {
	// 直接检查目标URL是否可访问
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.targetURL, nil)
	if err != nil {
		return fmt.Errorf("create check request failed: %w", err)
	}
	rsp, err := s.cc.cli.Do(req)
	if err != nil {
		return fmt.Errorf("check request failed: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode >= 400 {
		return fmt.Errorf("check request returned status %d", rsp.StatusCode)
	}
	return nil
}

func (s *URLSearcher) Search(ctx context.Context, num *number.Number) (*model.MovieMeta, bool, error) {
	ctx = api.InitContainer(ctx)
	ctx = meta.SetNumberId(ctx, num.GetNumberID())

	logger := logutil.GetLogger(ctx).With(
		zap.String("plugin", s.name),
		zap.String("url", s.targetURL),
		zap.String("number", num.GetNumberID()),
	)
	logger.Info("searching from forced URL")

	// 直接创建请求到指定URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.targetURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("create request failed: %w", err)
	}

	// 使用plugin装饰请求(添加headers, cookies等)
	if err := s.plg.OnDecorateRequest(ctx, req); err != nil {
		return nil, false, fmt.Errorf("decorate request failed: %w", err)
	}

	// 设置默认referer
	if len(req.Referer()) == 0 {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", req.URL.Scheme, req.URL.Host))
	}

	// 发起请求
	rsp, err := s.cc.cli.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("do request failed: %w", err)
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		logger.Warn("request returned non-OK status", zap.Int("status", rsp.StatusCode))
		return nil, false, nil
	}

	// 读取响应数据
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, false, fmt.Errorf("read response data failed: %w", err)
	}

	// 使用plugin解析数据
	movieMeta, decodeSucc, err := s.plg.OnDecodeHTTPData(ctx, data)
	if err != nil {
		return nil, false, fmt.Errorf("decode data failed: %w", err)
	}
	if !decodeSucc {
		logger.Warn("plugin failed to decode data from URL")
		return nil, false, nil
	}

	// 修复元数据
	s.fixMeta(ctx, req, movieMeta)

	// 下载并存储图片
	s.storeImageData(ctx, movieMeta)

	// 验证元数据
	if err := s.verifyMeta(movieMeta); err != nil {
		logger.Error("verify meta not pass, treat as not found", zap.Error(err))
		return nil, false, nil
	}

	movieMeta.ExtInfo.ScrapeInfo.Source = s.name
	movieMeta.ExtInfo.ScrapeInfo.DateTs = time.Now().UnixMilli()

	logger.Info("successfully scraped from forced URL", zap.String("title", movieMeta.Title))
	return movieMeta, true, nil
}

func (s *URLSearcher) verifyMeta(meta *model.MovieMeta) error {
	if meta.Cover == nil || len(meta.Cover.Name) == 0 {
		return fmt.Errorf("no cover")
	}
	if len(meta.Number) == 0 {
		return fmt.Errorf("no number")
	}
	if len(meta.Title) == 0 {
		return fmt.Errorf("no title")
	}
	if !meta.SwithConfig.DisableReleaseDateCheck && meta.ReleaseDate == 0 {
		return fmt.Errorf("no release_date")
	}
	return nil
}

func (s *URLSearcher) fixMeta(ctx context.Context, req *http.Request, mvmeta *model.MovieMeta) {
	if !mvmeta.SwithConfig.DisableNumberReplace {
		mvmeta.Number = meta.GetNumberId(ctx)
	}
	prefix := req.URL.Scheme + "://" + req.URL.Host
	if mvmeta.Cover != nil {
		s.fixSingleURL(req, &mvmeta.Cover.Name, prefix)
	}
	if mvmeta.Poster != nil {
		s.fixSingleURL(req, &mvmeta.Poster.Name, prefix)
	}
	for i := 0; i < len(mvmeta.SampleImages); i++ {
		s.fixSingleURL(req, &mvmeta.SampleImages[i].Name, prefix)
	}
}

func (s *URLSearcher) fixSingleURL(req *http.Request, input *string, prefix string) {
	if len(*input) >= 2 && (*input)[:2] == "//" {
		*input = req.URL.Scheme + ":" + *input
		return
	}
	if len(*input) >= 1 && (*input)[0] == '/' {
		*input = prefix + *input
		return
	}
}

func (s *URLSearcher) storeImageData(ctx context.Context, in *model.MovieMeta) {
	images := make([]string, 0, len(in.SampleImages)+2)
	if in.Cover != nil {
		images = append(images, in.Cover.Name)
	}
	if in.Poster != nil {
		images = append(images, in.Poster.Name)
	}
	for _, item := range in.SampleImages {
		images = append(images, item.Name)
	}
	imageDataMap := s.saveRemoteURLData(ctx, images)
	if in.Cover != nil {
		in.Cover.Key = imageDataMap[in.Cover.Name]
		if len(in.Cover.Key) == 0 {
			in.Cover = nil
		}
	}
	if in.Poster != nil {
		in.Poster.Key = imageDataMap[in.Poster.Name]
		if len(in.Poster.Key) == 0 {
			in.Poster = nil
		}
	}
	rebuildSampleList := make([]*model.File, 0, len(in.SampleImages))
	for _, item := range in.SampleImages {
		item.Key = imageDataMap[item.Name]
		rebuildSampleList = append(rebuildSampleList, item)
	}
	in.SampleImages = rebuildSampleList
}

func (s *URLSearcher) saveRemoteURLData(ctx context.Context, urls []string) map[string]string {
	rs := make(map[string]string, len(urls))
	for _, url := range urls {
		if len(url) == 0 {
			continue
		}
		logger := logutil.GetLogger(ctx).With(zap.String("url", url))
		key := hasher.ToSha1(url)
		if ok, _ := store.IsDataExist(ctx, key); ok {
			rs[url] = key
			continue
		}
		data, err := s.fetchImageData(ctx, url)
		if err != nil {
			logger.Error("fetch image data failed", zap.Error(err))
			continue
		}
		if err := store.PutData(ctx, key, data); err != nil {
			logger.Error("put image data to store failed", zap.Error(err))
		}
		rs[url] = key
	}
	return rs
}

func (s *URLSearcher) fetchImageData(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("make request for url:%s failed, err:%w", url, err)
	}
	if err := s.plg.OnDecorateMediaRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("decorate request failed: %w", err)
	}
	if len(req.Referer()) == 0 {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", req.URL.Scheme, req.URL.Host))
	}
	rsp, err := s.cc.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get url data failed, err:%w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get url data http code not ok, code:%d", rsp.StatusCode)
	}
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read url data failed, err:%w", err)
	}
	return data, nil
}
