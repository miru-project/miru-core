// ==MiruExtension==
// @name         V2 Watch/Mirror Example
// @package      v2watch
// @author       miru
// @license      MIT
// @lang         en
// @website      https://example.com
// @type         bangumi
// @apiVersion   2
// ==/MiruExtension==

// V2 contract demonstration for a bangumi (video) extension.
//
//   - watch(url) returns a LIST OF MIRRORS (proto.ExtensionWatch), NOT the final
//     link. The host maps the returned { groups: { title: [mirrors] } } object
//     into proto.ExtensionWatch so the frontend can let the user pick a mirror.
//
//   - mirror(url) receives the mirror URL the user picked and returns the FINAL
//     per-type watch object (here a bangumi stream: { type, url, headers }).
//
// This mirrors the Go/Scriggo V2 runtime's watch()->ExtensionWatch /
// mirror()->ExtensionBangumiWatch flow.

function decode_url(u) {
  return u;
}

async function watch(url) {
  const groups = {
    "Server 1": [
      { name: "Mirror 1", url: "https://mirror1.example.com/stream.m3u8", headers: {} },
      { name: "Mirror 2", url: "https://mirror2.example.com/stream.m3u8", headers: {} },
    ],
    "Server 2": [
      { name: "Backup", url: "https://backup.example.com/stream.m3u8", headers: {} },
    ],
  };
  return {
    groups: Object.keys(groups).map((title) => ({
      title: title,
      mirrors: groups[title],
    })),
  };
}

async function mirror(url) {
  const decoded = decode_url(url);
  return {
    type: "hls",
    url: decoded,
    headers: {
      "User-Agent":
        "Mozilla/5.0 (X11; Linux x86_64; rv:146.0) Gecko/10100101 Firefox/146.0",
      Accept: "*/*",
      "Accept-Language": "en-US,en;q=0.5",
      "Accept-Encoding": "gzip, deflate, br, zstd",
      "Sec-GPC": "1",
      Connection: "keep-alive",
      "Sec-Fetch-Dest": "empty",
      "Sec-Fetch-Mode": "cors",
      "Sec-Fetch-Site": "cross-site",
    },
  };
}
