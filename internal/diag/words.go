package diag

import (
	"strconv"
	"strings"

	"github.com/hidetzu/connect-doctor/internal/limits"
)

// Every sentence a human reads lives in this file (.claude/rules/go.md,
// CLAUDE.md § 4). Templates and handlers render; they do not compose.
//
// ⚠ Codes are the contract for programs; these sentences may be reworded.
// ⚠ Our refusals and our gaps are worded as ours, never as the target's
// failure (CLAUDE.md § 4-1).

// ObservedFromNote is said on every result (docs/adr/0001).
const ObservedFromNote = "ConnectDoctorのサーバから観測した結果です。あなたのPCからの接続結果ではありません。"

// Messages per code. A code missing here is a bug caught by a test.
var messages = map[string]string{
	"input.missing":             "URLを入力してください。",
	"input.malformed":           "URLとして解釈できませんでした。example.com や https://example.com/path のような形で入力してください。",
	"input.unsupported_scheme":  "ConnectDoctorが診断できるのは http:// と https:// のURLだけです。",
	"input.unsupported_port":    "ConnectDoctorが接続するのはポート80と443だけです。それ以外のポートは、ポートスキャンに使われないよう診断の対象外にしています。",
	"input.credentials":         "ユーザー名やパスワードを含むURLは診断しません。認証情報を取り除いて入力してください。",
	"input.local_name":          "localhost や .local など、特定のネットワークの中でしか意味を持たない名前は、ConnectDoctorの診断の対象外です。",
	"input.refused_address":     "プライベート・ループバック・リンクローカルなど、公開されていないアドレスには、ConnectDoctorは安全のため接続しません。",
	"input.idn_not_implemented": "日本語ドメインなどの国際化ドメイン名には、まだ対応していません。xn-- で始まる形式（Punycode）で入力してください。",

	"dns.not_found":       "このドメイン名は存在しないか、アドレス（A/AAAAレコード）が登録されていません。ホスト名の綴りと、DNSレコードの登録を確認してください。",
	"dns.timeout":         "DNSサーバから時間内に応答がありませんでした。ドメインのDNSサーバが応答していない可能性があります。",
	"dns.server_failure":  "DNSサーバがエラーを返しました。ドメインのDNS設定やDNSサーバに問題がある可能性があります。",
	"dns.refused_address": "このドメイン名は、公開されていないアドレス（プライベート・ループバックなど）に解決されました。ConnectDoctorは安全のため、このドメインには接続しません。",

	"tcp.refused":         "サーバには到達しましたが、このポートでは接続を受け付けていません（接続拒否）。サーバのプロセスが起動しているか、待ち受けているポートを確認してください。",
	"tcp.timeout":         "時間内に応答がありませんでした。ファイアウォールでパケットが破棄されているか、サーバが停止しているか、経路の途中で失われている可能性があります（ここからはこれらを区別できません）。",
	"tcp.unreachable":     "サーバへの経路がないと通知されました（到達不能）。サーバのアドレスや経路の設定を確認してください。",
	"tcp.no_route_family": "このドメインにはIPv6アドレスしかありませんが、ConnectDoctorのサーバはIPv6で外部に接続できません。これはConnectDoctor側の制約で、サイト側の問題とは限りません。",
	"tcp.refused_address": "ConnectDoctorの接続先検査が、このアドレスへの接続を止めました。安全のため接続しません。",
	"tcp.failed":          "分類できない種類のエラーでした。詳細のエラーを確認してください。",

	"tls.cert_expired":       "サーバ証明書の有効期間外です（期限切れ、またはまだ有効になっていません）。証明書を更新してください。日付は詳細に表示しています。",
	"tls.cert_untrusted":     "サーバ証明書を信頼できません。自己署名の証明書か、中間証明書がサーバに設定されていない可能性があります。",
	"tls.cert_name_mismatch": "サーバ証明書が、このホスト名のものではありません。証明書に含まれる名前は詳細に表示しています。",
	"tls.handshake_failed":   "TLSの交渉に失敗しました。サーバが対応しているTLSのバージョンや暗号方式が合わない可能性があります。",
	"tls.timeout":            "TCP接続はできましたが、TLSの応答が時間内に返りませんでした。",
	"tls.not_tls":            "このポートはTLSではない応答を返しました。https:// ではなく http:// で待ち受けている可能性があります。",

	"http.timeout":            "TLS（またはTCP）までは成功しましたが、HTTPリクエストへの応答が時間内に返りませんでした。サーバのアプリケーションが応答していない可能性があります。",
	"http.no_response":        "リクエストを送りましたが、サーバは何も返さずに接続を閉じました。サーバのアプリケーションやリバースプロキシを確認してください。",
	"http.malformed_response": "サーバの応答がHTTPとして読めませんでした（形式が正しくないか、ヘッダが大きすぎます）。",

	"http.redirect_refused":   "リダイレクト先に、ConnectDoctorは安全のため接続しませんでした。",
	"http.too_many_redirects": "リダイレクトが多すぎるため、たどるのを止めました。リダイレクトがループしている可能性があります。",

	"server.rate_limited": "短い時間に診断が集中したため、ConnectDoctorは一時的に受け付けを止めています。少し待ってから、もう一度お試しください。",

	"server.target_rate_limited": "この診断先への診断が短い時間に集中しているため、ConnectDoctorは接続を控えました。診断先の負担を避けるための制限で、サイト側の問題ではありません。少し待ってから、もう一度お試しください。",

	"server.busy": "ただいま混み合っています。しばらくしてから、もう一度お試しください。",
}

// Message returns the sentence for code.
func Message(code string) string { return messages[code] }

var stepNames = map[string]string{
	StepDNS:  "DNSの名前解決",
	StepTCP:  "TCP接続",
	StepTLS:  "TLSハンドシェイク",
	StepHTTP: "HTTPリクエスト",
}

// StatusLabel is the short word beside a step on the page.
var statusLabels = map[Status]string{
	StatusOK:             "成功",
	StatusFailed:         "失敗",
	StatusRefused:        "診断対象外",
	StatusSkipped:        "未実施",
	StatusNotApplicable:  "対象外",
	StatusNotImplemented: "未実装",
}

// StatusLabel returns the word for s.
func StatusLabel(s Status) string { return statusLabels[s] }

func summaryRefusedInput(code string) string {
	return "ConnectDoctorはこのURLを診断しませんでした。" + Message(code)
}

func summaryFailed(step, code string) string {
	return "ConnectDoctorのサーバからは、" + stepNames[step] + "に失敗しました。" + Message(code)
}

func summaryRefusedStep(code string) string {
	return "ConnectDoctorはこのURLへの接続を行いませんでした。" + Message(code)
}

func summaryIncomplete(lastOK string, notImplemented []string) string {
	names := make([]string, len(notImplemented))
	for i, st := range notImplemented {
		names[i] = strings.ToUpper(st)
	}
	gap := strings.Join(names, "・") + "の診断はまだ実装されていないため、接続できるかどうかはまだ判定していません。"
	if lastOK == "" {
		return gap
	}
	return "ConnectDoctorのサーバからは、" + stepNames[lastOK] + "に成功しました。" + gap
}

// summaryOK says what the status means. ⚠ Owner decision
// (hidetzu/connect-doctor#4): any status is a successful connection; the
// sentence says what the server answered.
func summaryOK(code int, https bool) string {
	layers := "DNS・TCP・TLS・HTTP"
	if !https {
		layers = "DNS・TCP・HTTP"
	}
	head := "ConnectDoctorのサーバからは、" + layers + "のすべてに成功しました。"
	st := "サーバはステータス" + strconv.Itoa(code)
	var body string
	switch {
	case code >= 500:
		body = "接続はできていますが、" + st + "を返しました。サーバ側（アプリケーション）でエラーが起きています。"
	case code >= 400:
		body = "接続はできていますが、" + st + "を返しました。URLのパスや、アクセス権限を確認してください。"
	case code >= 300:
		body = st + "（リダイレクト）を返しました。"
	default:
		body = st + "を返しました。"
	}
	return head + body + "あなたの環境から繋がらない場合は、あなた側のネットワーク（プロキシ・DNS・ファイアウォールなど）を確認してください。"
}

func summaryRedirectRefused(hop int, reason string) string {
	return "ConnectDoctorは、リダイレクト先（" + strconv.Itoa(hop) + "番目のURL）に接続しませんでした。" + Message(reason)
}

func summaryTooManyRedirects(limit int) string {
	return "ConnectDoctorのサーバからは、リダイレクトを" + strconv.Itoa(limit) + "回たどっても終わりませんでした。" + Message("http.too_many_redirects")
}

// summaryAfterRedirects says that the answer is about a later URL.
func summaryAfterRedirects(redirects int) string {
	if redirects == 0 {
		return ""
	}
	return "（リダイレクトを" + strconv.Itoa(redirects) + "回たどった先の結果です。）"
}

func summaryTargetLimited(hop int) string {
	if hop == 1 {
		return Message(CodeTargetLimited)
	}
	return "リダイレクト先（" + strconv.Itoa(hop) + "番目のURL）への診断が短い時間に集中しているため、ConnectDoctorは接続を控えました。診断先の負担を避けるための制限で、サイト側の問題ではありません。少し待ってから、もう一度お試しください。"
}

// ---- The page's headline card (hidetzu/connect-doctor#25, candidate A) ----
//
// ⚠ Colour encodes state, never layer: State returns one of ok / warn / fail /
// neutral. A refusal by ConnectDoctor is neutral, never red or yellow — it is
// ours, not the target's (CLAUDE.md § 4-1, owner decision B on #25).

var layerName = map[string]string{StepDNS: "DNS", StepTCP: "TCP", StepTLS: "TLS", StepHTTP: "HTTP"}

// State is the page's colour class for a result.
func State(r Result) string {
	switch r.Conclusion.Status {
	case ConclusionFailed:
		return "fail"
	case ConclusionRefused:
		return "neutral"
	}
	if lastStatusCode(r) >= 400 {
		return "warn"
	}
	return "ok"
}

// Headline is the first line of the result: where it stopped, or that it did not.
func Headline(r Result) string {
	c := r.Conclusion
	switch {
	case c.Code == CodeTargetLimited:
		return "接続を控えました"
	case c.Code == "http.redirect_refused":
		return "リダイレクト先で止めました"
	case c.Code == "http.too_many_redirects":
		return "リダイレクトが終わりません"
	case strings.HasPrefix(c.Code, "input."):
		return "このURLは診断しませんでした"
	case c.Status == ConclusionFailed:
		return layerName[c.FailedStep] + " で止まっています"
	case c.Status == ConclusionRefused:
		return layerName[c.FailedStep] + " で止めました"
	}
	if s := lastStatusCode(r); s >= 400 {
		return "接続できています — サーバは " + strconv.Itoa(s) + " を返しました"
	}
	return "すべての層を通りました（" + strconv.Itoa(lastStatusCode(r)) + "）"
}

// Cause is the one line under the headline.
func Cause(r Result) string {
	c := r.Conclusion
	if c.Code == "http.redirect_refused" {
		for _, h := range r.Hops[1:] {
			if h.Code != "" {
				return Message(h.Code)
			}
			for _, s := range h.Steps {
				if s.Status == StatusRefused {
					return Message(s.Code)
				}
			}
		}
	}
	if c.Code != "" {
		return Message(c.Code)
	}
	switch s := lastStatusCode(r); {
	case s >= 500:
		return "サーバ側（アプリケーション）でエラーが起きています。"
	case s >= 400:
		return "URLのパスや、アクセス権限を確認してください。"
	}
	if len(r.Hops) > 1 {
		return "リダイレクトを" + strconv.Itoa(len(r.Hops)-1) + "回たどった先の結果です。あなたの環境から繋がらない場合は、あなた側のネットワークを確認してください。"
	}
	return "少なくとも ConnectDoctor のサーバからは接続できています。あなたの環境から繋がらない場合は、あなた側のネットワーク（プロキシ・DNS・ファイアウォールなど）を確認してください。"
}

func lastStatusCode(r Result) int {
	if len(r.Hops) == 0 {
		return 0
	}
	for _, s := range r.Hops[len(r.Hops)-1].Steps {
		if s.Step == StepHTTP && s.Detail != nil {
			return s.Detail.StatusCode
		}
	}
	return 0
}

// ---- What the page says about itself (hidetzu/connect-doctor#41) ----

// Tagline is the owner's one-line description, under the header.
const Tagline = "URLがなぜ繋がらないのかを、DNS → TCP → TLS → HTTP の順に調べて答えます。"

// SourceURL is where the code lives.
const SourceURL = "https://github.com/hidetzu/connect-doctor"

// PrivacyNote says what happens to a URL, and ⚠ only what is true of the
// running service: the platform's request log keeps the URL for a while (Cloud
// Run records the full request URL; owner decision 2026-10-11 to keep that log
// and say so), the result cache (hidetzu/connect-doctor#28), and the hostname
// logged when a limit refuses (docs/adr/0010). The cache's duration is said as
// "a short while", not in seconds (owner decision 2026-10-11).
// ⚠ Change the code or the deployment, change this sentence.
func PrivacyNote() string {
	return "入力したURLは、サーバのアクセスログにしばらく残ります。同じURLの再表示のために結果を短い間だけメモリに保持し、" +
		"利用回数の制限に掛かったときだけホスト名を記録します。"
}

// RateLimited is the sentence for a visitor over their own limit
// (hidetzu/connect-doctor#43): which limit, and how many seconds until the
// next check. reason is internal/ratelimit's Reason; the numbers come from
// internal/limits, never written into the sentence by hand.
func RateLimited(reason string, seconds int) string {
	var which string
	switch reason {
	case "burst":
		which = "続けて診断できるのは" + strconv.Itoa(limits.ClientBurst) + "回までです。"
	case "hour":
		which = "診断できるのは1時間に" + strconv.Itoa(limits.ClientPerHour) + "回までです。"
	case "day":
		which = "診断できるのは1日に" + strconv.Itoa(limits.ClientPerDay) + "回までです。"
	case "concurrent":
		return "前の診断がまだ終わっていません。終わってから、もう一度お試しください。"
	default:
		return Message("server.rate_limited")
	}
	return "あなたの診断回数が上限に達しました。" + which + "あと " + strconv.Itoa(seconds) + " 秒で、もう一度診断できます。"
}

// ---- What /about says (owner decision 2026-10-11) ----

// AboutSection is one heading of /about and the sentences under it.
type AboutSection struct {
	Title string
	Items []string
	Code  string // a command to show under the sentences, or empty
}

// AboutTitle is the page's own name.
const AboutTitle = "ConnectDoctorについて"

// AboutLede is the one line under the heading: what this page is for.
const AboutLede = "ConnectDoctorが何を調べ、どう読み、何を断るかを説明します。"

// About is the visitor's rendering of docs/PRODUCT.md § 4–6 and docs/SPEC.md
// § 2, in the order CLAUDE.md § 4-1 asks for: what this does first, then what
// it declines, each with its reason and what to do instead.
// base is the public origin for the API example, or empty for a relative path.
// ⚠ Every number comes from internal/limits. ⚠ When PRODUCT, SPEC § 2 or a
// limit changes, these sentences change with it — a test pins the numbers.
func About(base string) []AboutSection {
	return []AboutSection{
		{Title: "何を調べるか", Items: []string{
			"URLを1つ受け取り、ConnectDoctorのサーバから次の4つの層を順に試して、最初に止まった層とその意味を一文で答えます。",
			"DNS — ホスト名をIPアドレスに解決します。名前が登録されていない、DNSサーバが応答しない、といった失敗はここで分かります。",
			"TCP — 解決したアドレスの80番または443番ポートに接続します。接続拒否やタイムアウトはここで分かります。",
			"TLS — https:// のURLでは、TLSハンドシェイクを行い、サーバ証明書をホスト名に対して検証します。期限切れ、名前の不一致、信頼できない発行者はここで分かります。http:// のURLではこの層は対象外です。",
			"HTTP — GETリクエストを1回送り、ステータス行とヘッダを読みます。応答が返らない、HTTPとして読めない、はここで分かります。4xx や 5xx のステータスは「サーバが応答した」のでこの層としては成功で、その意味は結論に書きます。",
			"リダイレクト（301/302/303/307/308）はたどります。たどった先のURLも最初の4層からやり直し、1つずつ結果を並べます。たどるのは" + strconv.Itoa(limits.RedirectHops) + "回までです。",
		}},
		{Title: "結果の読み方", Items: []string{
			"結論は、最初に止まった層とその意味を一文で示します。ある層で止まると、その先の層は試しません（— と表示します）。",
			"各層の印は 4 種類です。✓ はその層を通過、✕ はその層で失敗、⊘ はConnectDoctorが自分の方針で接続しなかった印（診断先の問題ではありません）、— は試さなかった印です。",
			"色は状態を表し、層は表しません。緑はすべて通過、黄はサーバが 4xx・5xx を返した、赤はどこかの層で失敗、灰はConnectDoctorが接続しなかった、です。",
			"同じURLは、短い間だけ前回の結果をそのまま表示します。そのときは「N秒前の診断結果です」と表示し、再診断のリンクを出します。",
			"「詳細」を開くと、解決したアドレス、各層の所要時間、証明書の内容、ステータス行とヘッダの抜粋が見えます。",
		}},
		{Title: "どこから観測しているか", Items: []string{
			"すべての結果はConnectDoctorのサーバから観測したもので、あなたのPCからの接続結果ではありません。",
			"ここから繋がっても、あなたから繋がるとは限りません。社内プロキシ、VPN、ローカルのDNS設定、手元のファイアウォールは、ここからは見えません。",
			"ここから失敗しても、サイトが落ちているとは限りません。診断先がConnectDoctorの接続元を遮断している、経路が違う、ということもあります。",
		}},
		{Title: "診断しないもの", Items: []string{
			"ConnectDoctorが答える質問は「このURLはなぜ繋がらないか」だけです。速さの計測、脆弱性やTLS設定の採点、稼働監視、経路の追跡、所有者の照会は行いません。それぞれ別の質問で、専用のツールがあります。",
			"接続するのはポート80と443だけです。ほかのポートを診断すると、ConnectDoctorが他人のサーバのポートスキャンに使えてしまうためです。",
			"プライベート・ループバック・リンクローカルなど、公開されていないアドレスには接続しません。ホスト名がそうしたアドレスに解決された場合も、そのホスト名ごと接続しません。リダイレクト先についても同じです。",
			"localhost や .local など、特定のネットワークの中でしか意味を持たない名前、ユーザー名やパスワードを含むURLも診断しません。",
			"日本語ドメインなどの国際化ドメイン名には、まだ対応していません。xn-- で始まる形式（Punycode）で入力してください。",
			"HTTPは HTTP/1.1 で1回だけ要求します。HTTP/2 でしか応答しないサーバは、まだ正しく診断できません。",
		}},
		{Title: "利用回数の制限", Items: []string{
			"診断先のサーバに負担をかけないため、1人が診断できる回数には上限があります。続けて" + strconv.Itoa(limits.ClientBurst) + "回まで、1時間に" + strconv.Itoa(limits.ClientPerHour) + "回まで、1日に" + strconv.Itoa(limits.ClientPerDay) + "回までで、同時に進められる診断は" + strconv.Itoa(limits.ClientConcurrent) + "つです。上限に達したときは、あと何秒待てばよいかを表示します。",
			"同じ診断先への診断が、誰からであっても短い時間に集中したときは、その診断先への接続を控えます。これは診断先の負担を避けるための制限で、サイト側の問題ではありません。",
		}},
		{Title: "プログラムから使う", Items: []string{
			"ページと同じ結果を、JSONでも返します。診断先、各層の結果と所要時間、結論、観測地点の注記が含まれます。",
			"入力したURLを診断しなかったときは 400、あなたの利用回数の上限に達したときは 429（Retry-After ヘッダに待ち秒数）、混み合っているときは 503 を返します。",
		}, Code: "curl '" + base + "/api/check?url=https://example.com'"},
	}
}
