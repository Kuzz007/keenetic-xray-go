package keeneticroute

import "strings"

// catalogData is the curated domain/CIDR list for commonly-blocked
// services, ported verbatim from route_data() in the reference
// project's xray-keenetic-routes.sh -- this is curated production data,
// not logic, so it's copied as-is rather than regenerated. Each line is
// "<group>|<domain-or-cidr>"; groups are routed through the Proxy
// interface via Keenetic's dns-proxy/object-group mechanism (domains)
// -- bare CIDR entries are listed for reference/parity with the source
// but are not currently applied as routes here (see SyncRoutes).
const catalogData = `apple|12dagenkerstbijitunes.com
apple|12diasdepresentesdeitunes.com
apple|12diasderegalosdeitunes.cl
apple|12diasderegalosdeitunes.co
apple|12diasderegalosdeitunes.co.cr
apple|12diasderegalosdeitunes.co.ve
apple|12diasderegalosdeitunes.com
apple|12diasderegalosdeitunes.com.co
apple|12diasderegalosdeitunes.com.hn
apple|12diasderegalosdeitunes.com.ni
apple|12diasderegalosdeitunes.com.ve
apple|12diasderegalosdeitunes.cr
apple|12diasderegalosdeitunes.gt
apple|12diasderegalosdeitunes.hn
apple|12diasderegalosdeitunes.pe
apple|12joursdecadeauxdeitunes.com
apple|12joursdecadeauxdeitunes.fr
apple|aaplimg.com
apple|apple-appstore.cn
apple|apple-cloudkit.com
apple|apple-dns.cn
apple|apple-dns.com
apple|apple-dns.com.cn
apple|apple-dns.net
apple|apple-icloud.cn
apple|apple-itunes.cn
apple|apple-livephotoskit.com
apple|apple-mapkit.com
apple|apple.com
apple|appleappstore.cn
apple|appleappstore.net
apple|appleappstore.tv
apple|applecare.berlin
apple|applecare.cc
apple|applecare.com
apple|applecare.eu
apple|applecare.hamburg
apple|applecare.wang
apple|appledns.cn
apple|appledns.com.cn
apple|appleicloud.cn
apple|applemx-icloud.com
apple|appstore.co.id
apple|appstore.com
apple|appstore.com.br
apple|appstore.hk
apple|appstore.my
apple|appstore.ph
apple|appstoreapple.cn
apple|carekit.org
apple|cdn-apple.com
apple|foundationdb.com
apple|foundationdb.net
apple|foundationdb.org
apple|icloud-apple.cn
apple|icloud-content.com
apple|icloud-sandbox.com
apple|icloud.ch
apple|icloud.cl
apple|icloud.com
apple|icloud.com.cn
apple|icloud.de
apple|icloud.ee
apple|icloud.fi
apple|icloud.fr
apple|icloud.hu
apple|icloud.ie
apple|icloud.is
apple|icloud.jp
apple|icloud.lv
apple|icloud.net.cn
apple|icloud.org
apple|icloud.pt
apple|icloud.ro
apple|icloud.se
apple|icloud.si
apple|icloud.sk
apple|icloud.vn
apple|icloudapple.cn
apple|ios-icloud.com
apple|itunes-apple.cn
apple|itunes-nocookie.com
apple|itunes-radio.net
apple|itunes12days.com
apple|itunes12tage.com
apple|itunesessentials.com
apple|itunesfestivals.com
apple|itunesiradio.com
apple|ituneslatino.com
apple|itunesmatch.com
apple|itunesmusicstore.com
apple|itunesparty.com
apple|itunesradio.com
apple|itunesu.com
apple|itunesu.net
apple|los12diasderegalosdeitunes.es
apple|macosforge.org
apple|mzstatic.com
apple|researchandcare.org
apple|researchkit.tv
apple|shazam.com
apple|swift.org
apple|webkit.org
apple|webkitgtk.org
apple|wpewebkit.org
apple|wwwicloud.com
apple|wwwitunes.com
apple|17.0.0.0/8
apple|cloudflare
apple|103.21.244.0/22
apple|103.22.200.0/22
apple|103.31.4.0/22
apple|104.16.0.0/13
apple|104.24.0.0/14
apple|108.162.192.0/18
apple|131.0.72.0/22
apple|141.101.64.0/18
apple|162.158.0.0/15
apple|172.64.0.0/13
apple|173.245.48.0/20
apple|188.114.96.0/20
apple|190.93.240.0/20
apple|197.234.240.0/22
apple|198.41.128.0/17
cloudflare|104.16.0.0/13
cloudflare|104.16.0.0/12
cloudflare|104.24.0.0/14
cloudflare|162.158.0.0/15
cloudflare|172.64.0.0/13
cloudflare|188.114.96.0/20
cloudflare|103.21.244.0/22
cloudflare|103.22.200.0/22
cloudflare|103.31.4.0/22
cloudflare|108.162.192.0/18
cloudflare|131.0.72.0/22
cloudflare|141.101.64.0/18
cloudflare|173.245.48.0/20
cloudflare|190.93.240.0/20
cloudflare|197.234.240.0/22
cloudflare|198.41.128.0/17
discord|dis.gd
discord|discord-activities.com
discord|discord.co
discord|discord.com
discord|discord.design
discord|discord.dev
discord|discord.gg
discord|discord.gift
discord|discord.gifts
discord|discord.media
discord|discord.new
discord|discord.store
discord|discord.tools
discord|discordactivities.com
discord|discordapp.com
discord|discordapp.io
discord|discordapp.net
discord|discordcdn.com
discord|discordmerch.com
discord|discordpartygames.com
discord|discordsays.com
discord|discordstatus.com
discord|138.128.136.0/21
discord|162.159.0.0/16
discord|172.65.202.19/32
discord|34.0.0.0/15
discord|34.2.0.0/15
discord|35.192.0.0/12
discord|35.208.0.0/12
discord|5.200.14.128/25
discord|66.22.192.0/18
gemini|ai.google.dev
gemini|ai.studio
gemini|aistudio.google.com
gemini|bard.google.com
gemini|deepmind.com
gemini|deepmind.google
gemini|gemini.google
gemini|gemini.google.com
gemini|generativeai.google
gemini|jules.google
gemini|jules.google.com
gemini|labs.google
gemini|labs.google.com
gemini|notebooklm.google
gemini|notebooklm.google.com
gemini|opal.google
gemini|opal.google.com
gemini|stitch.withgoogle.com
meta|aboutfacebook.com
meta|facebook.com
meta|facebook.net
meta|facebook.org
meta|facebookmail.com
meta|fb.com
meta|fb.gg
meta|fb.me
meta|fb.watch
meta|fbcdn.com
meta|fbcdn.net
meta|fbsbx.com
meta|fbsbx.net
meta|instagram.com
meta|cdninstagram.com
meta|ig.me
meta|igcdn.com
meta|messenger.com
meta|m.me
meta|meta.ai
meta|meta.com
meta|oculus.com
meta|oculuscdn.com
meta|oculusvr.com
meta|threads.com
meta|threads.net
meta|wa.me
meta|whatsapp.com
meta|whatsapp.net
meta|whatsapp.org
meta|workplace.com
meta|31.13.24.0/21
meta|31.13.64.0/18
meta|45.64.40.0/22
meta|57.141.0.0/20
meta|57.141.16.0/22
meta|57.141.20.0/23
meta|57.144.0.0/14
meta|66.220.144.0/20
meta|69.63.176.0/20
meta|69.171.224.0/19
meta|74.119.76.0/22
meta|102.132.96.0/20
meta|102.132.112.0/24
meta|102.132.115.0/24
meta|102.132.116.0/23
meta|102.132.119.0/24
meta|102.132.120.0/23
meta|102.132.123.0/24
meta|102.132.125.0/24
meta|102.132.126.0/24
meta|102.221.188.0/22
meta|103.4.96.0/22
meta|129.134.0.0/17
meta|129.134.130.0/24
meta|129.134.132.0/24
meta|129.134.135.0/24
meta|129.134.136.0/22
meta|129.134.140.0/24
meta|129.134.143.0/24
meta|129.134.144.0/24
meta|129.134.147.0/24
meta|129.134.148.0/23
meta|129.134.154.0/23
meta|129.134.156.0/22
meta|129.134.160.0/22
meta|129.134.164.0/23
meta|129.134.168.0/21
meta|129.134.176.0/20
meta|129.134.194.0/24
meta|129.134.196.0/24
meta|157.240.0.0/17
meta|157.240.128.0/23
meta|157.240.131.0/24
meta|157.240.132.0/24
meta|157.240.134.0/24
meta|157.240.136.0/23
meta|157.240.139.0/24
meta|157.240.140.0/24
meta|157.240.156.0/22
meta|157.240.169.0/24
meta|157.240.170.0/24
meta|157.240.175.0/24
meta|157.240.177.0/24
meta|157.240.179.0/24
meta|157.240.181.0/24
meta|157.240.182.0/23
meta|157.240.184.0/21
meta|157.240.192.0/18
meta|163.70.128.0/17
meta|163.77.132.0/23
meta|163.77.136.0/23
meta|163.114.128.0/20
meta|173.252.64.0/18
meta|179.60.192.0/22
meta|185.60.216.0/22
meta|185.89.216.0/22
meta|199.201.64.0/22
meta|204.15.20.0/22
telegram|cdn-telegram.org
telegram|comments.app
telegram|contest.com
telegram|fragment.com
telegram|graph.org
telegram|quiz.directory
telegram|t.me
telegram|tdesktop.com
telegram|telegra.ph
telegram|telegram-cdn.org
telegram|telegram.dog
telegram|telegram.me
telegram|telegram.org
telegram|telegram.space
telegram|telesco.pe
telegram|tg.dev
telegram|ton.org
telegram|tx.me
telegram|91.105.192.0/23
telegram|91.108.4.0/22
telegram|91.108.8.0/21
telegram|91.108.16.0/21
telegram|91.108.56.0/22
telegram|95.161.64.0/20
telegram|149.154.160.0/20
telegram|185.76.151.0/24
tiktok|byteimg.com
tiktok|byteoversea.com
tiktok|byteoversea.net
tiktok|bytetcdn.com
tiktok|ibytedtos.com
tiktok|ibyteimg.com
tiktok|ipstatp.com
tiktok|isnssdk.com
tiktok|muscdn.com
tiktok|musical.ly
tiktok|pstatp.com
tiktok|sgsnssdk.com
tiktok|tiktok.com
tiktok|tiktokcdn-eu.com
tiktok|tiktokcdn-in.com
tiktok|tiktokcdn-us.com
tiktok|tiktokcdn.com
tiktok|tiktokd.org
tiktok|tiktokglobalshop.com
tiktok|tiktokv.com
tiktok|tiktokv.eu
tiktok|tiktokw.eu
tiktok|tlivecdn.com
tiktok|ttlivecdn.com
tiktok|ttoversea.net
tiktok|ttoverseaus.net
tiktok|ttwstatic.com
tiktok|101.45.0.0/24
tiktok|101.45.3.0/24
tiktok|101.45.4.0/24
tiktok|101.45.5.0/24
tiktok|101.45.16.0/24
tiktok|101.45.192.0/22
tiktok|101.45.196.0/24
tiktok|101.45.248.0/22
tiktok|103.136.220.0/22
tiktok|130.44.212.0/24
tiktok|130.44.214.0/23
tiktok|139.177.225.0/24
tiktok|139.177.227.0/24
tiktok|139.177.233.0/24
tiktok|139.177.235.0/24
tiktok|139.177.238.0/24
tiktok|139.177.240.0/21
tiktok|139.177.248.0/24
tiktok|147.160.176.0/24
tiktok|147.160.177.0/24
tiktok|147.160.180.0/24
tiktok|147.160.190.0/24
tiktok|199.103.24.0/23
tiktok|202.52.240.0/21
youtube|googlevideo.com
youtube|i.ytimg.com
youtube|manifest.googlevideo.com
youtube|m.youtube.com
youtube|music.youtube.com
youtube|s.youtube.com
youtube|studio.youtube.com
youtube|tv.youtube.com
youtube|wide-youtube.l.google.com
youtube|www.youtube.com
youtube|youtu.be
youtube|youtube-nocookie.com
youtube|youtube.com
youtube|youtube.googleapis.com
youtube|youtubeeducation.com
youtube|youtubeembeddedplayer.googleapis.com
youtube|youtubei.googleapis.com
youtube|youtubekids.com
youtube|yt.be
youtube|yt3.ggpht.com
youtube|yt3.googleusercontent.com
youtube|yt4.ggpht.com
youtube|ytimg.com
youtube|ytimg.googleusercontent.com
youtube|ytimg.l.google.com
youtube|64.233.160.0/19
youtube|66.102.0.0/20
youtube|66.249.64.0/19
youtube|72.14.192.0/18
youtube|74.125.0.0/16
youtube|108.170.192.0/18
youtube|108.177.0.0/17
youtube|142.250.0.0/15
youtube|172.217.0.0/16
youtube|172.253.0.0/16
youtube|173.194.0.0/16
youtube|192.178.0.0/15
youtube|208.65.152.0/22
youtube|208.117.224.0/19
youtube|209.85.128.0/17
youtube|216.58.192.0/19
youtube|216.239.32.0/19`

// CatalogEntry is one "group|domain-or-cidr" line from the catalog.
type CatalogEntry struct {
	Group string
	Item  string
}

// Catalog parses catalogData into entries, in file order.
func Catalog() []CatalogEntry {
	lines := strings.Split(catalogData, "\n")
	entries := make([]CatalogEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		group, item, ok := strings.Cut(line, "|")
		if !ok || group == "" || item == "" {
			continue
		}
		entries = append(entries, CatalogEntry{Group: group, Item: item})
	}
	return entries
}

// isDomain reports whether item looks like a domain (as opposed to a
// bare CIDR/IP entry, which SyncRoutes doesn't apply as a dns-proxy
// route -- Keenetic's dns-proxy object-group mechanism routes by domain
// name resolution, not by matching an IP already in a packet).
func isDomain(item string) bool {
	if item == "" {
		return false
	}
	for _, r := range item {
		if r >= '0' && r <= '9' || r == '.' || r == '/' {
			continue
		}
		return true
	}
	// All-digits/dots/slashes: an IPv4 literal or CIDR, not a domain.
	return false
}

// Groups returns the distinct group names with at least one domain
// entry, in catalog order, deduplicated.
func Groups() []string {
	seen := make(map[string]bool)
	var groups []string
	for _, e := range Catalog() {
		if !isDomain(e.Item) {
			continue
		}
		if seen[e.Group] {
			continue
		}
		seen[e.Group] = true
		groups = append(groups, e.Group)
	}
	return groups
}
