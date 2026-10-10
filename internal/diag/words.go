package diag

import "strings"

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
	"input.malformed":           "URLとして解釈できませんでした。http:// または https:// から始まる形で入力してください。",
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

const notImplementedMessage = "このステップはまだ実装されていません。"

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
