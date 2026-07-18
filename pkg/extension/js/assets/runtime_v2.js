// ################# These lines reserve for go fmt.sprintf ################# //
const pkg = '%s';
const name = '%s';
const website = '%s';
// ################# These lines reserve for go fmt.sprintf ################# // 
class XPathNode {
  constructor(content, selector) {
    this.content = content;
    this.selector = selector;
  }
  async excute(fun) {
    // return await handlePromise("queryXPath$className", JSON.stringify([this.content, this.selector, fun]));
  }
  get attr() {
    return this.excute("attr");
  }
  get attrs() {
    return this.excute("attrs");
  }
  get text() {
    return this.excute("text");
  }
  get allHTML() {
    return this.excute("allHTML");
  }
  get outerHTML() {
    return this.excute("outerHTML");
  }
}

// 在 load 中注册的 keys
const settingKeys = [];
const Miru = {
  pkg: pkg,
  name: name,
  website: website,
  request: async (url, options) => {
    options = options || {};
    options.headers = options.headers || {};
    const miruUrl = options.headers["Miru-Url"] || website;
    options.method = options.method || "get";
    if (options.headers["Miru-Url"]) {
      delete options.headers["Miru-Url"];
    }
    const res = await jsRequest(miruUrl + url, options);
    try {
      return JSON.parse(res);
    } catch (e) {
      return res;
    }
  },
  rawRequest: async (url, options) => {
    options = options || {};
    options.headers = options.headers || {};
    options.method = options.method || "GET";
    // const message = await handlePromise("rawRequest$className", JSON.stringify([url, options, "${extension.package}"]));
    const message = await jsRequest(url,options)
    try {
      return JSON.parse(message);
    } catch (e) {
      return message;
    }
  },

  // Cross-function / cross-call variable store. Because the goja VM is disposed
  // after every execution, long-lived state that an extension wants to share
  // between its functions (e.g. a value computed once in load() and read later
  // by latest()/search()/detail()) must live outside the VM. These delegate to
  // the native saveCache/getCache functions registered by the host. Values are
  // always strings.
  saveCache: (key, value) => {
    return saveCache(key, String(value));
  },
  getCache: (key) => {
    return getCache(key);
  },

}
var latest = () => {
  throw new Error("not implement latest");
}
var search = () => {
  throw new Error("not implement search");
}
var createFilter = () => {
  throw new Error("not implement createFilter");
}
var detail = () => {
  throw new Error("not implement detail");
}
// V2 watch() MUST return a LIST OF MIRRORS, NOT the final link. The host maps
// the returned value into proto.ExtensionWatch and the frontend lets the user
// pick one mirror, then calls mirror(url) to resolve it.
//
// Accepted shapes (host accepts both):
//   - { groups: [{ title, mirrors: [{ name, url, headers }] }] }   (array form)
//   - { groups: { "Server 1": [{ name, url, headers }], ... } }    (object form,
//     keyed by group title -- recommended, see example below)
//
// Example:
//   var watch = (url) => {
//     const groups = {
//       "Server 1": [{ name: "Mirror 1", url: "https://.../1", headers: {} }],
//       "Server 2": [{ name: "Mirror 2", url: "https://.../2", headers: {} }],
//     };
//     return {
//       groups: Object.keys(groups).map((title) => ({
//         title: title,
//         mirrors: groups[title],
//       })),
//     };
//   };
var watch = () => {
  throw new Error("not implement watch");
}

// V2 mirror(url) receives the mirror URL the user picked from watch() and MUST
// return the FINAL per-type watch object for that mirror (one of the proto
// watch shapes dictated by the extension's @type). This is exactly the shape
// that V1 watch() returned -- "V2 mirror is like V1 watch on all resources".
// For bangumi that is:
//   { type: "hls" | "mp4" | "torrent" | "magnet", url, headers: { ... } }
// NOTE the `type` field is the CONTENT type (hls/mp4/torrent/magnet), NEVER
// the extension type ("bangumi"). The host routes the result into the matching
// MirrorResponse variant.
//
// DEFAULT BEHAVIOUR: if an extension does NOT define mirror(), the host
// simply echoes the chosen mirror URL back as the link itself (a pass-through).
// Define mirror() to do real work -- decode the URL, add headers, resolve to
// a torrent, etc. -- when you need more than the raw mirror link.
//
// Example:
//   var mirror = (url) => {
//     const decoded = decode_url(url);
//     return {
//       type: "hls",
//       url: decoded,
//       headers: {
//         "User-Agent": "Mozilla/5.0 ...",
//         "Referer": "https://example.com/",
//       },
//     };
//   };
var mirror = (url) => {
  throw new Error("not implement mirror");
}
var checkUpdate = () => {
  throw new Error("not implement checkUpdate");
}
async function load() { }

var throwError = (message) => {
  throw new Error(message);
}