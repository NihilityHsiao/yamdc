package pornhub

type pornhubVideoData struct {
	//Event     string `json:"event"`
	VideoData struct {
		//VideoOrientation      string `json:"video_orientation"`
		//VideoSegment          string `json:"video_segment"`
		//VideoProduction       string `json:"video_production"`
		//HdVideo               string `json:"hd_video"`
		PornStarsInVideo string `json:"pornstars_in_video"`
		// 字符串, 英文逗号分隔, 但这里的tag不是中文翻译的
		CategoriesInVideo string `json:"categories_in_video"`
		//VideoGeoJapan         string `json:"video_geo_japan"`
		// example: Amateur Model
		VideoUploader         string `json:"video_uploader"`
		VideoDuration         string `json:"video_duration"`
		VideoDatePublished    string `json:"video_date_published"`
		LanguageSpokenInVideo string `json:"language_spoken_in_video"`
		// 实际的上传者
		VideoUploaderName string `json:"video_uploader_name"`
	} `json:"videodata"`
}
