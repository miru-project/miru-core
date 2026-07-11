// ==MiruExtension==
// @name         Example
// @version      v0.1.0
// @author       AUTHOR_NAME
// @lang         all
// @license      MIT
// @icon         YOUR_LINK_TO_ICON
// @package      example.org
// @type         bangumi
// @webSite      WEB_LINK
// @nsfw         false
// ==/MiruExtension==

package example

// ExtensionListItem represents a search result item.
type ExtensionListItem struct {
	Title  string
	URL    string
	Cover  string
	Update string
	Image  string
	Type   string
}

// ExtensionDetail represents detailed content information.
type ExtensionDetail struct {
	Title       string
	URL         string
	Cover       string
	Image       string
	Type        string
	Description string
	Desc        string
	Chapters    []ExtensionEpisodeGroup
}

// ExtensionEpisodeGroup represents a group of episodes.
type ExtensionEpisodeGroup struct {
	Title string
	URLs  []string
}

// ExtensionWatch represents watch/stream information.
type ExtensionWatch struct {
	Title  string
	URL    string
	Type   string
	Pages  []string
	Groups []ExtensionMirrorGroup
}

// ExtensionMirrorGroup represents a group of mirrors.
type ExtensionMirrorGroup struct {
	Title   string
	Mirrors []ExtensionMirror
}

// ExtensionMirror represents a mirror/alternative URL.
type ExtensionMirror struct {
	Name string
	URL  string
}

// Search searches for content by keyword.
func Search(pkg, kw string, page int, filter string) ([]ExtensionListItem, error) {
	results := []ExtensionListItem{
		{
			Title:  "Example Result 1",
			URL:    "https://example.com/1",
			Cover:  "https://example.com/1.jpg",
			Update: "2024-01-01",
			Image:  "https://example.com/1.jpg",
			Type:   "manga",
		},
		{
			Title:  "Example Result 2",
			URL:    "https://example.com/2",
			Cover:  "https://example.com/2.jpg",
			Update: "2024-01-02",
			Image:  "https://example.com/2.jpg",
			Type:   "bangumi",
		},
	}
	return results, nil
}

// Latest returns the latest content for a package.
func Latest(pkg string, page int) ([]ExtensionListItem, error) {
	results := []ExtensionListItem{
		{
			Title:  "Latest Example 1",
			URL:    "https://example.com/latest/1",
			Cover:  "https://example.com/latest/1.jpg",
			Update: "2024-01-03",
			Image:  "https://example.com/latest/1.jpg",
			Type:   "manga",
		},
	}
	return results, nil
}

// Detail returns detailed information about a content item.
func Detail(pkg, url string) (*ExtensionDetail, error) {
	return &ExtensionDetail{
		Title: "Example Detail",
		Desc:  "Example description",
		URL:   url,
	}, nil
}

// Watch returns watch/stream information for a content item.
func Watch(pkg, url string) (*ExtensionWatch, error) {
	return &ExtensionWatch{
		Title: "Example Watch",
		URL:   url,
		Groups: []ExtensionMirrorGroup{
			{
				Title: "Group 1",
				Mirrors: []ExtensionMirror{
					{
						Name: "Mirror 1",
						URL:  url,
					},
				},
			},
		},
	}, nil
}

// Mirror returns mirror/alternative URLs for a content item.
func Mirror(pkg, url string) ([]ExtensionMirror, error) {
	return []ExtensionMirror{
		{
			Name: "Mirror 1",
			URL:  url,
		},
	}, nil
}
