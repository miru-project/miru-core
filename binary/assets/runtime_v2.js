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
  // listCookies: async () => {
  //   return await handlePromise("listCookies$className", "");
  // },
  // setCookie: async (cookie) => {
  //   return await handlePromise("setCookie$className", cookie);
  // },
  // saveData: async (key, data) => {
  //   try { await handlePromise("saveData$className", JSON.stringify([key, data])); return true; } catch (e) { return false; }
  // },
  // snackbar: (message) => {
  //   return handlePromise("snackbar$className", JSON.stringify([message]));
  // },
  // getData: async (key) => {
  //   return await handlePromise("getData$className", JSON.stringify([key]));
  // },
  // queryXPath: (content, selector) => {
  //   return new XPathNode(content, selector);
  // },
  // registerSetting: async (settings) => {
  //   console.log(JSON.stringify([settings]));
  //   settingKeys.push(settings.key);
  //   return await handlePromise("registerSetting$className", JSON.stringify([settings]));
  // },
  // getSetting: async (key) => {
  //   return await handlePromise("getSetting$className", JSON.stringify([key]));
  // },
  // convert:async (data,from,to)=>{
  //   return await handlePromise("convert$className",JSON.stringify([JSON.stringify(data),from,to]));
  // }
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
var watch = () => {
  throw new Error("not implement watch");
}
var checkUpdate = () => {
  throw new Error("not implement checkUpdate");
}
async function load() { }

var throwError = (message) => {
  throw new Error(message);
}
// const handlePromise = async (channelName, message) => {
//   // const waitForChange = new Promise(resolve => {
//   //   DartBridge.setHandler(channelName, async (arg) => {
//   //     resolve(arg);
//   //   })
//   // });
//   // DartBridge.sendMessage(channelName, message);
//   // return await waitForChange
// }
// const stringify = async (callback) => {
//   const data = await callback();
//   return typeof data === "object" ? JSON.stringify(data, 0, 2) : data;
// }
