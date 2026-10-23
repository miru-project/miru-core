// ==MiruExtension==
// @name         V1 Class Example
// @version      v0.0.1
// @author       Miru
// @lang         all
// @license      MIT
// @package      v1class
// @type         bangumi
// @icon         https://example.com/icon.png
// @webSite      https://example.com
// @nsfw         false
// ==/MiruExtension==

export default class extends Extension {
  async search(kw, page, filter) {
    return [
      { title: "Search Result", url: "https://example.com/1", cover: "https://example.com/1.jpg" },
    ];
  }

  async latest(page) {
    return [
      { title: "Latest Result", url: "https://example.com/2", cover: "https://example.com/2.jpg" },
    ];
  }

  async detail(url) {
    return {
      title: "Detail Title",
      url: url,
      cover: "https://example.com/cover.jpg",
      desc: "A test extension",
      tags: ["Action", "Adventure"],
      episodes: [
        {
          title: "Streams",
          urls: [{ name: "360p", url: "https://example.com/stream.mp4" }],
        },
      ],
    };
  }

  async watch(url) {
    return {
      type: "mp4",
      url: url,
    };
  }
}
