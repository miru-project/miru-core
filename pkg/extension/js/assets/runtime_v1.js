// ################# These lines reserve for go fmt.sprintf ################# //
const pkg = '%s';
const name = '%s';
const website = '%s';
// ################# These lines reserve for go fmt.sprintf ################# //

class V1Element {
  constructor(document) {
    this.document = document;
    this.content = document.outerHTML;
  }


  querySelector(selector) {
    this.document= this.document.querySelector(selector);
    // console.log(this.document)
    this.content = this.document.outerHTML;
    return this;
  }

  removeSelector(selector) {
    const removeNode = this.document.querySelector(selector);
    if (removeNode && removeNode.parentNode) {
      removeNode.parentNode.removeChild(removeNode);
    }
    this.content = this.document.documentElement.outerHTML;
    return this;
  }

  getAttributeText(attr) {
    return this.document.getAttribute(attr);
  }

  querySelectorAll(selector) {
    const nodes = this.document.querySelectorAll(selector);
    return nodes.map(function (e) {
      const c = e
      c.content = e.outerHTML;
      return c;
    });
  }


  get text() {
    return this.document.textContent;
  }

  get outerHTML() {
    return this.document.outerHTML;
  }

  get innerHTML() {
    return this.document.innerHTML;
  }
}




class Extension {
  constructor(webSite) {
    this.webSite = webSite;
  }

  settingKeys = [];

  async request(url, options) {
    options = options || {};
    options.headers = options.headers || {};
    const miruUrl = options.headers["Miru-Url"] || this.webSite;
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
  }

  async rawRequest(url, options) {
    options = options || {};
    options.headers = options.headers || {};
    options.method = options.method || "GET";
    const message = await jsRequest(url,options)
    try {
      return JSON.parse(message);
    } catch (e) {
      return message;
    }
  }

  querySelector(content, selector) {
    const {parseHTML} = require("linkedom")
    const { document } = parseHTML(content);
    const e = new V1Element(document).querySelector(selector)
    return e;
  }

  queryXPath(content, selector) {
    return new XPathNode(content, selector, this.extension);
  }
  querySelectorAll(content, selector) {
    const {parseHTML} = require("linkedom")
    const { document } = parseHTML(content);
    const e = document.querySelectorAll(selector).map(function (e) {
      const c = new V1Element(e)
      return c;
    })
    return e;
  }

  async getAttributeText(content, selector, attr) {
    const {parseHTML} = require("linkedom")
    const { document } = parseHTML(content);
    const node = document.querySelector(selector);
    return node ? node.getAttribute(attr) : null;
  }

  popular(page) {
    throw new Error("not implement popular");
  }

  latest(page) {
    throw new Error("not implement latest");
  }

  search(kw, page, filter) {
    throw new Error("not implement search");
  }

  createFilter(filter) {
    throw new Error("not implement createFilter");
  }

  detail(url) {
    throw new Error("not implement detail");
  }

  // V1 watch(url) returns the FINAL watchable resource DIRECTLY (no mirror
  // step). The returned shape follows the extension's @type: for bangumi a
  // per-type object { type: "hls"|"mp4"|"torrent"|"magnet", url, headers },
  // for manga a list of page URLs, for fikushon a list of text content, for
  // all a combined object. NOTE the bangumi `type` is the CONTENT type
  // (hls/mp4/torrent/magnet), never the extension type ("bangumi"). This is
  // exactly the shape V2 mirror() produces -- V1 watch() and V2 mirror() are
  // the same final-link step, just reached differently. The host resolves any
  // magnet:/torrent link into a downloadable handle. V1 has no mirror()
  // function.
  watch(url) {
    throw new Error("not implement watch");
  }

  tags(url) {
    throw new Error("not implement watch");
  }

  checkUpdate(url) {
    throw new Error("not implement checkUpdate");
  }

  async getSetting(key) {
    return getSetting(key);
  }

  async registerSetting(settings) {
    console.log("Register setting:", settings);
    // this.settingKeys.push(settings.key);
    return await registerSetting(settings);
  }

  async setSetting(key, value) {
  }

  async listCookies(url) {
    return await getCookies(url);
  }

  async setCookies(url, cookies) {
    return await setCookies(url, cookies);
  }

  async load() { }
}
// Cross-call state for V1 is captured and rebuilt by the HOST (see
// pkg/extension/js/instance_state.go): the goja VM is disposed after every
// call, and the host keeps a structural copy of the `ext` instance's own
// properties (functions, objects, Date/RegExp/Map/Set/TypedArray/BigInt all
// preserved) which it re-instantiates on the fresh VM at the start of the next
// call. No JS helper is involved.
// Miru object exposed to V1 extensions. It is the SAME surface as the V2
// runtime's Miru object, so an extension can use Miru.request/rawRequest
// identically on both API versions. Cross-call state needs no helper: the host
// snapshots the `ext` instance's own properties after every call and restores
// them at the start of the next one (see the comment above). pkg/name/website
// are injected by the host (the sprintf placeholders at the top of this file).
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
    const message = await jsRequest(url, options);
    try {
      return JSON.parse(message);
    } catch (e) {
      return message;
    }
  },
}