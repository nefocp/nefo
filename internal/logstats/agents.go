package logstats

import "bytes"

// Family names with a special meaning; the UI translates them.
const (
	NoAgent    = ""          // the request had no User-Agent header
	OtherAgent = "Other"     // a browser or program not in the lists below
	OtherBot   = "Other bot" // a robot whose name could not be read
)

type family struct {
	needle string // lowercase
	name   string
}

// Robots and programs, checked first, in this order. Every match counts as
// a bot request.
var botFamilies = []family{
	{"googlebot", "Googlebot"},
	{"adsbot-google", "Googlebot"},
	{"mediapartners-google", "Googlebot"},
	{"google-inspectiontool", "Googlebot"},
	{"googleother", "Googlebot"},
	{"bingbot", "Bingbot"},
	{"bingpreview", "Bingbot"},
	{"yandexbot", "YandexBot"},
	{"yandex.com/bots", "YandexBot"},
	{"baiduspider", "Baiduspider"},
	{"duckduckbot", "DuckDuckBot"},
	{"applebot", "Applebot"},
	{"yahoo! slurp", "Yahoo Slurp"},
	{"seznambot", "SeznamBot"},
	{"petalbot", "PetalBot"},
	{"ahrefsbot", "AhrefsBot"},
	{"semrushbot", "SemrushBot"},
	{"mj12bot", "MJ12bot"},
	{"dotbot", "DotBot"},
	{"dataforseobot", "DataForSeoBot"},
	{"bytespider", "Bytespider"},
	{"gptbot", "GPTBot"},
	{"chatgpt-user", "ChatGPT"},
	{"oai-searchbot", "ChatGPT"},
	{"claudebot", "ClaudeBot"},
	{"claude-user", "ClaudeBot"},
	{"perplexitybot", "PerplexityBot"},
	{"ccbot", "CCBot"},
	{"amazonbot", "Amazonbot"},
	{"facebookexternalhit", "Facebook"},
	{"meta-externalagent", "Facebook"},
	{"twitterbot", "Twitterbot"},
	{"linkedinbot", "LinkedInBot"},
	{"slackbot", "Slackbot"},
	{"discordbot", "Discordbot"},
	{"telegrambot", "TelegramBot"},
	{"whatsapp", "WhatsApp"},
	{"uptimerobot", "UptimeRobot"},
	{"pingdom", "Pingdom"},
	{"let's encrypt", "Let's Encrypt"},
	{"headlesschrome", "Headless Chrome"},
	{"lighthouse", "Lighthouse"},
	{"curl/", "curl"},
	{"wget/", "Wget"},
	{"python-requests", "Python Requests"},
	{"python-urllib", "Python urllib"},
	{"python-httpx", "Python httpx"},
	{"aiohttp", "Python aiohttp"},
	{"scrapy", "Scrapy"},
	{"go-http-client", "Go HTTP client"},
	{"okhttp", "OkHttp"},
	{"apache-httpclient", "Apache HttpClient"},
	{"java/", "Java"},
	{"libwww-perl", "Perl LWP"},
	{"guzzlehttp", "Guzzle"},
	{"node-fetch", "Node.js"},
	{"undici", "Node.js"},
	{"axios/", "axios"},
	{"postmanruntime", "Postman"},
	{"l9scan", "LeakIX"},
	{"zgrab", "ZGrab"},
	{"masscan", "Masscan"},
	{"nmap", "Nmap"},
	{"nuclei", "Nuclei"},
	{"censysinspect", "Censys"},
	{"wpscan", "WPScan"},
	{"sqlmap", "sqlmap"},
	{"nikto", "Nikto"},
}

// Words that mark any other robot.
var botWords = []string{"bot", "crawl", "spider", "slurp", "scanner", "fetcher", "monitor", "headless"}

// Browsers, checked in this order: most browsers also say "Chrome" and
// "Safari".
var browserFamilies = []family{
	{"edg/", "Edge"},
	{"edga/", "Edge"},
	{"edgios/", "Edge"},
	{"edge/", "Edge"},
	{"opr/", "Opera"},
	{"opera", "Opera"},
	{"samsungbrowser", "Samsung Internet"},
	{"yabrowser", "Yandex Browser"},
	{"ucbrowser", "UC Browser"},
	{"vivaldi", "Vivaldi"},
	{"firefox/", "Firefox"},
	{"fxios/", "Firefox"},
	{"crios/", "Chrome"},
	{"chrome/", "Chrome"},
	{"chromium/", "Chrome"},
	{"safari/", "Safari"},
	{"msie ", "Internet Explorer"},
	{"trident/", "Internet Explorer"},
}

// classify returns the simplified family of a User-Agent and whether it is a
// robot or a program rather than a person's browser. A missing User-Agent is
// counted as a bot: browsers always send one. lower is scratch space.
//
// Besides the known names, a User-Agent is a robot's if it contains one of
// botWords or a contact address ("+https://..."), or if it is not a browser's
// and either does not start with "Mozilla/" (programs name themselves first)
// or says "compatible;" ("Mozilla/5.0 (compatible; ExampleBot/1.0)").
func classify(agent []byte, lower *[]byte) (name string, bot bool) {
	if len(agent) == 0 {
		return NoAgent, true
	}
	l := append((*lower)[:0], agent...)
	for i, c := range l {
		if c >= 'A' && c <= 'Z' {
			l[i] = c + ('a' - 'A')
		}
	}
	*lower = l
	for _, f := range botFamilies {
		if bytes.Contains(l, []byte(f.needle)) {
			return f.name, true
		}
	}
	browser := ""
	for _, f := range browserFamilies {
		if bytes.Contains(l, []byte(f.needle)) {
			browser = f.name
			break
		}
	}
	word := -1
	for _, w := range botWords {
		if i := bytes.Index(l, []byte(w)); i >= 0 {
			word = i
			break
		}
	}
	mozilla := bytes.HasPrefix(l, []byte("mozilla/"))
	compatible := bytes.Index(l, []byte("compatible;"))
	if word >= 0 || bytes.Contains(l, []byte("+http")) || browser == "" && (!mozilla || compatible >= 0) {
		return botName(agent, compatible, mozilla, word), true
	}
	if browser != "" {
		return browser, false
	}
	return OtherAgent, false
}

func isWordChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || isDigit(c) || c == '-' || c == '_' || c == '.'
}

// botName finds the name of an unknown robot: the product after
// "compatible; ", the first word of a User-Agent that does not start with
// "Mozilla/", or the word around a match of botWords.
func botName(agent []byte, compatible int, mozilla bool, word int) string {
	valid := func(name []byte) bool {
		// A web address such as "http://example.com/bot.html" says nothing.
		return len(name) >= 3 && len(name) <= 40 && !bytes.ContainsAny(name, ".")
	}
	if compatible >= 0 {
		s := compatible + len("compatible;")
		for s < len(agent) && agent[s] == ' ' {
			s++
		}
		e := s
		for e < len(agent) && isWordChar(agent[e]) {
			e++
		}
		if name := bytes.Trim(agent[s:e], "-_."); valid(name) {
			return string(name)
		}
	}
	if !mozilla {
		e := 0
		for e < len(agent) && isWordChar(agent[e]) {
			e++
		}
		if name := bytes.Trim(agent[:e], "-_."); valid(name) {
			return string(name)
		}
	}
	if word >= 0 {
		s, e := word, word
		for s > 0 && isWordChar(agent[s-1]) {
			s--
		}
		for e < len(agent) && isWordChar(agent[e]) {
			e++
		}
		if name := bytes.Trim(agent[s:e], "-_."); valid(name) {
			return string(name)
		}
	}
	return OtherBot
}
