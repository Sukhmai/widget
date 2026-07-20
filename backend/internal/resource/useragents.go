package resource

// User-Agent strings for outbound fetches. Sites fall into two opposite camps,
// so picking the right one per fetcher matters:
//
//   - identifyingUA: for APIs whose policy requires honest identification with
//     contact info (e.g. Wikimedia). A generic or browser-spoofed UA gets
//     rejected with HTTP 403.
//     https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy
//
//   - browserUA: for sites that gate bots at the edge (e.g. Letterboxd behind
//     Cloudflare). Sending an honest bot UA here gets blocked — the exact
//     opposite of identifyingUA — so we present as a desktop browser instead.
const (
	identifyingUA = "WidgetBot/0.1 (https://github.com/sukhmai/widget; sukhmai.kapur@gmail.com) Go-http-client/1.1"

	browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)
