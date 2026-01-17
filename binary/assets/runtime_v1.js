class Element {
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

  // execute(fun) {
  //   if (fun === "text") {
  //     return this.document.textContent;
  //   }
  //   if (fun === "outerHTML") {
  //     return this.document.outerHTML;
  //   }
  //   if (fun === "innerHTML") {
  //     return this.document.innerHTML;
  //   }
  //   // Add more as needed
  //   return null;
  // }

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
// class XPathNode {
//   constructor(content, selector,extension) {
//     this.content = content;
//     this.selector = selector;
//     this.extension=extension;
//   }

//   async execute(fun) {
//     return await sendMessage(
//       "queryXPath",
//       JSON.stringify([this.content, this.selector, fun])
//     );
//   }

//   get attr() {
//     return this.execute("attr");
//   }

//   get attrs() {
//     return this.execute("attrs");
//   }

//   get text() {
//     return this.execute("text");
//   }

//   get allHTML() {
//     return this.execute("allHTML");
//   }

//   get outerHTML() {
//     return this.execute("outerHTML");
//   }
// }




class Extension {
  constructor(webSite) {
    this.webSite = webSite;
  }

  //package = this.extension.package;
  //name = this.extension.name;
  // 在 load 中注册的 keys
  settingKeys = [];

  async request(url, options) {
    options = options || {};
    options.headers = options.headers || {};
    const miruUrl = options.headers["Miru-Url"] || this.webSite;
    // println("miruUrl",options.headers["Miru-Url"])
    // println("miruUrl: " + miruUrl);
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
    const { document } = parseHTML(content);
    // console.log(document)
    // console.log(selector)
    // console.log(document.querySelector(selector)) 
    const e = new Element(document).querySelector(selector)
    return e;
  }

  queryXPath(content, selector) {
    return new XPathNode(content, selector, this.extension);
  }
  querySelectorAll(content, selector) {
    const { document } = parseHTML(content);
    const e = document.querySelectorAll(selector).map(function (e) {
      const c = new Element(e)
      return c;
    })
    return e;
  }

  async getAttributeText(content, selector, attr) {
    // console.log(content)
    const { document } = parseHTML(content);
    const node = document.querySelector(selector);
    // console.log(node)
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
// const stringify = async (callback) => {
//   const data = await callback();
//   return typeof data === "object" ? JSON.stringify(data, 0, 2) : data;
// }