package jsExtension

type Ext struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	License     string   `json:"license"`
	Lang        string   `json:"lang"`
	Icon        string   `json:"icon"`
	Pkg         string   `json:"package"`
	Website     string   `json:"webSite"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Api         string   `json:"api"`
	Error       string   `json:"error,omitempty"`
	Context     *string
	WatchType   string `json:"type"`
}

type ExtensionListItem struct {
	Cover   string            `json:"cover"`
	Title   string            `json:"title"`
	Update  string            `json:"update"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type ExtensionEpisode struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type ExtensionEpisodeGroup struct {
	Title string             `json:"title"`
	Urls  []ExtensionEpisode `json:"urls"`
}

type ExtensionDetail struct {
	Title    string                  `json:"title"`
	Cover    string                  `json:"cover"`
	Desc     string                  `json:"desc"`
	Episodes []ExtensionEpisodeGroup `json:"episodes"`
	Headers  map[string]string       `json:"headers"`
}

type ExtensionListItems []ExtensionListItem

type ExtensionBangumiWatchSubtitle struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	URL      string `json:"url"`
}

type ExtensionBangumiWatch struct {
	Type       string                          `json:"type"`
	URL        string                          `json:"url"`
	Subtitles  []ExtensionBangumiWatchSubtitle `json:"subtitles"`
	Headers    map[string]string               `json:"headers"`
	AudioTrack string                          `json:"audioTrack"`
}

type ExtensionMangaWatch struct {
	URLs    []string          `json:"urls"`
	Headers map[string]string `json:"headers"`
}

type ExtensionFikushonWatch struct {
	Content  []string `json:"content"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
}

type ExtensionFilter struct {
	Title         string            `json:"title"`
	Min           int               `json:"min"`
	Max           int               `json:"max"`
	DefaultOption string            `json:"default"`
	Options       map[string]string `json:"options"`
}
