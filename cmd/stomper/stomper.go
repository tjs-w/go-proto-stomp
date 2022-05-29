package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"

	"github.com/tjs-w/go-proto-stomp/pkg/stomp"
)

type stateContext struct {
	messages    []string
	currTxAlias string
	aliasToTx   map[string]*stomp.Transaction
	client      *stomp.ClientHandler
	destToSubs  map[string]*stomp.Subscription
}

var ctx stateContext

func init() {
	ctx.destToSubs = map[string]*stomp.Subscription{}
	ctx.aliasToTx = map[string]*stomp.Transaction{}
}

var (
	errPrint  = color.New(color.FgRed, color.Bold)
	infoPrint = color.New(color.FgBlue, color.Italic)
	okPrint   = color.New(color.FgGreen)
)

func errorMsg(s string) {
	_, _ = errPrint.Println(s)
}

func okMsg(s string) {
	_, _ = okPrint.Println(s)

}

var suggestions = []prompt.Suggest{
	// STOMP Commands
	{"connect", "Connect to a STOMP broker"},
	{"disconnect", "Disconnect from the STOMP broker"},
	{"subscribe", "Subscribe to a destination on broker"},
	{"unsubscribe", "Unsubscribe from a destination on broker"},
	{"send", "Send a message to a destination on broker"},
	{"tx_leave", "Leave an ongoing transaction"},
	{"tx_enter", "Enter an ongoing transaction"},
	{"tx_begin", "Start a transaction"},
	{"tx_abort", "Abort the transaction"},
	{"tx_commit", "Commit the transaction"},
	{"session", "Show session info"},
	{"read", "Read received messages"},
	{"help", "Display commands help"},
	{"bye", "End session by disconnecting from the STOMP broker"},
	{"quit", "End session by disconnecting from the STOMP broker"},
}

var headerSuggestions = []prompt.Suggest{
	// HTTP Header
	{"Accept", "Acceptable response media type"},
	{"Accept-Charset", "Acceptable response charsets"},
	{"Accept-Encoding", "Acceptable response content codings"},
	{"Accept-Language", "Preferred natural languages in response"},
	{"ALPN", "Application-layer protocol negotiation to use"},
	{"Alt-Used", "Alternative host in use"},
	{"Authorization", "Authentication information"},
	{"Cache-Control", "Directives for caches"},
	{"Connection", "Connection options"},
	{"Content-Encoding", "Content codings"},
	{"Content-Language", "Natural languages for content"},
	{"Content-Length", "Anticipated size for payload body"},
	{"Content-Location", "Where content was obtained"},
	{"Content-MD5", "Base64-encoded MD5 sum of content"},
	{"Content-Type", "Content media type"},
	{"Cookie", "Stored cookies"},
	{"Date", "Datetime when message was originated"},
	{"Depth", "Applied only to resource or its members"},
	{"DNT", "Do not track user"},
	{"Expect", "Expected behaviors supported by server"},
	{"Forwarded", "Proxies involved"},
	{"From", "Sender email address"},
	{"Host", "Target URI"},
	{"HTTP2-Settings", "HTTP/2 connection parameters"},
	{"If", "Request condition on state tokens and ETags"},
	{"If-Match", "Request condition on target resource"},
	{"If-Modified-Since", "Request condition on modification date"},
	{"If-None-Match", "Request condition on target resource"},
	{"If-Range", "Request condition on Range"},
	{"If-Schedule-Tag-Match", "Request condition on Schedule-Tag"},
	{"If-Unmodified-Since", "Request condition on modification date"},
	{"Max-Forwards", "Max number of times forwarded by proxies"},
	{"MIME-Version", "Version of MIME protocol"},
	{"Origin", "Origin(s} issuing the request"},
	{"Pragma", "Implementation-specific directives"},
	{"Prefer", "Preferred server behaviors"},
	{"Proxy-Authorization", "Proxy authorization credentials"},
	{"Proxy-Connection", "Proxy connection options"},
	{"Range", "Request transfer of only part of data"},
	{"Referer", "Previous web page"},
	{"TE", "Transfer codings willing to accept"},
	{"Transfer-Encoding", "Transfer codings applied to payload body"},
	{"Upgrade", "Invite server to upgrade to another protocol"},
	{"User-Agent", "User agent string"},
	{"Via", "Intermediate proxies"},
	{"Warning", "Possible incorrectness with payload body"},
	{"WWW-Authenticate", "Authentication scheme"},
	{"X-Csrf-Token", "Prevent cross-site request forgery"},
	{"X-CSRFToken", "Prevent cross-site request forgery"},
	{"X-Forwarded-For", "Originating client IP address"},
	{"X-Forwarded-Host", "Original host requested by client"},
	{"X-Forwarded-Proto", "Originating protocol"},
	{"X-Http-Method-Override", "Request method override"},
	{"X-Requested-With", "Used to identify Ajax requests"},
	{"X-XSRF-TOKEN", "Prevent cross-site request forgery"},
}

func tcpConnect(host string, port string) {
	var err error

	keyPrint := color.New(color.FgRed, color.Bold, color.Italic).SprintFunc()
	valPrint := color.New(color.FgYellow).SprintFunc()
	bodyPrint := func(b []byte) []byte {
		return []byte(color.New(color.FgBlue).Sprint(string(b)))
	}

	ctx.client, err = stomp.StartTcpClient(host, port, func(message *stomp.UserMessage) {
		sb := strings.Builder{}
		for k, v := range message.Headers {
			sb.WriteString(fmt.Sprintf("%s:%s\n", keyPrint(k), valPrint(v)))
		}
		sb.Write(bodyPrint(message.Body))
		ctx.messages = append(ctx.messages, sb.String())
	})
	if err != nil {
		fmt.Println(err)
	}
}

func livePrefix() (string, bool) {
	ps := fmt.Sprintf("stomper [%d] ", len(ctx.messages))
	if ctx.currTxAlias != "" {
		ps += "tx:" + ctx.currTxAlias + " "
	}
	return ps + "⋙  ", true
}

func executor(line string) {
	if line == "" {
		return
	}
	in := strings.Fields(strings.TrimSpace(line))
	cmd := in[0]

	switch cmd {
	case "session":
		showSessionInfo()
	case "send":
		handleSend(in)
	case "tx_begin":
		handleTxBegin(in)
	case "tx_abort":
		handleTxAbort()
	case "tx_commit":
		handleTxCommit()
	case "tx_leave":
		handleTxLeave()
	case "tx_enter":
		handleTxEnter(in)
	case "subscribe":
		handleSubscribe(in)
	case "unsubscribe":
		handleUnsubscribe(in)
	case "connect":
		handleConnect(in)
	case "read":
		handleRead(in)
	case "help":
		showHelp()
	case "disconnect", "bye", "quit":
		handleDisconnect()
		_, _ = infoPrint.Println("See ya!")
		os.Exit(0)
	}
}

func showHelp() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, line := range suggestions {
		_, _ = infoPrint.Fprintf(w, "%s\t\t%s\n", line.Text, line.Description)
	}
	w.Flush()
}

func handleTxEnter(in []string) {
	if len(in) < 2 {
		errorMsg("Must provide tx alias to enter")
		return
	}
	if _, ok := ctx.aliasToTx[in[1]]; !ok {
		errorMsg("No such tx alias present")
		return
	}
	ctx.currTxAlias = in[1]
	okMsg("Entering tx " + ctx.currTxAlias + ": " + ctx.aliasToTx[ctx.currTxAlias].TxID)
}

func handleTxLeave() {
	ctx.currTxAlias = ""
	okMsg("Switched out of transaction")
}

func handleTxBegin(in []string) {
	if len(in) < 2 {
		errorMsg("Must provide a tx name as arg following the command. Usage: tx_being <tx alias>")
		return
	}
	if ctx.currTxAlias != "" {
		errorMsg("Must leave current tx to start a new one. Use command 'tx_leave'")
		return
	}
	alias := in[1]
	tx, err := ctx.client.BeginTransaction()
	if err != nil {
		errorMsg(err.Error())
		return
	}
	ctx.aliasToTx[alias] = tx
	ctx.currTxAlias = alias
	okMsg("Transaction began, " + alias + ": " + tx.TxID)
}

func handleTxCommit() {
	if ctx.currTxAlias == "" {
		errorMsg("Not inside a transaction")
		return
	}
	tx := ctx.aliasToTx[ctx.currTxAlias]
	if err := tx.CommitTransaction(); err != nil {
		errorMsg(err.Error())
		return
	}
	okMsg("Transaction commit, " + ctx.currTxAlias + ":" + tx.TxID)
	delete(ctx.aliasToTx, ctx.currTxAlias)
	ctx.currTxAlias = ""
}

func handleTxAbort() {
	if ctx.currTxAlias == "" {
		errorMsg("Not inside a transaction")
		return
	}
	tx := ctx.aliasToTx[ctx.currTxAlias]
	if err := tx.AbortTransaction(); err != nil {
		errorMsg(err.Error())
		return
	}
	okMsg("Transaction aborted, " + ctx.currTxAlias + ":" + tx.TxID)
	delete(ctx.aliasToTx, ctx.currTxAlias)
	ctx.currTxAlias = ""
}

func showSessionInfo() {
	_, _ = infoPrint.Println("Session ID:", ctx.client.SessionID)
	_, _ = infoPrint.Println("Unread Messages:", len(ctx.messages))
	var l []string
	for k := range ctx.destToSubs {
		l = append(l, k)
	}
	_, _ = infoPrint.Println("Subscriptions:", l)
	_, _ = infoPrint.Println("Current Transaction:", ctx.currTxAlias)
	l = l[:0]
	for k := range ctx.aliasToTx {
		l = append(l, k)
	}
	_, _ = infoPrint.Println("Ongoing Transactions:", l)
}

func handleUnsubscribe(in []string) {
	if len(in) < 2 {
		errorMsg("Missing destination")
		return
	}
	dest := in[1]
	if err := ctx.destToSubs[dest].Unsubscribe(); err != nil {
		errorMsg(err.Error())
		return
	}
	delete(ctx.destToSubs, dest)
	okMsg("Unsubscribed from " + dest)
}

func handleSubscribe(in []string) {
	if len(in) < 2 {
		errorMsg("Missing destination")
		return
	}
	dest := in[1]
	subs, err := ctx.client.Subscribe(dest, stomp.HdrValAckAuto)
	if err != nil {
		errorMsg(err.Error())
		return
	}
	ctx.destToSubs[dest] = subs
	okMsg("Subscribed to " + dest)
}

func handleRead(in []string) {
	if len(ctx.messages) == 0 {
		return
	}

	all := false
	if len(in) > 1 && in[1] == "-a" {
		all = true
	}

	var poll int
	fmt.Println("――――――――――――――――――――――――――――――――――――――――――――――――――――――――――――")
	for i, m := range ctx.messages {
		fmt.Println(m)
		fmt.Println("――――――――――――――――――――――――――――――――――――――――――――――――――――――――――――")
		poll = i
		if !all {
			break
		}
	}
	ctx.messages = ctx.messages[poll+1:]
}

func handleSend(in []string) {
	var opts struct {
		Headers []string `short:"h" description:"Add custom header"`
		File    string   `short:"f" description:"File name to read body from"`
		Dest    string   `short:"d" description:"Destination on the broker"`
	}
	var err error
	in = in[1:]
	if in, err = flags.NewParser(&opts, flags.None).ParseArgs(in); err != nil {
		errorMsg(err.Error())
		return
	}

	if opts.Dest == "" {
		errorMsg("Missing destination")
		return
	}

	h := map[string]string{}
	for _, v := range opts.Headers {
		kv := strings.Split(v, ":")
		if len(kv) != 2 {
			errorMsg("Faulty header: " + v + ", must contain single ':'")
			return
		}
		h[kv[0]] = kv[1]
	}

	// Pick current transaction for message being sent
	if ctx.currTxAlias != "" {
		h[string(stomp.HdrKeyTransaction)] = ctx.aliasToTx[ctx.currTxAlias].TxID
	}

	if err = ctx.client.Send(opts.Dest, []byte(strings.Join(in, " ")), "plain/text", h); err != nil {
		errorMsg(err.Error())
		return
	}
	okMsg("Message sent")
}

func handleConnect(in []string) {
	if len(in) < 3 {
		errorMsg("Missing host and port")
		return
	}
	tcpConnect(in[1], in[2])
	if err := ctx.client.Connect(false); err != nil {
		errorMsg(err.Error())
		return
	}
	okMsg("Connected to " + in[1] + ":" + in[2])
}

func handleDisconnect() {
	if err := ctx.client.Disconnect(); err != nil {
		errorMsg(err.Error())
	}
}

func completion(d prompt.Document) []prompt.Suggest {
	args := strings.Fields(strings.TrimSpace(d.TextBeforeCursor()))
	// COMMAND
	if len(args) == 1 {
		return prompt.FilterFuzzy(suggestions, d.GetWordBeforeCursorWithSpace(), true)
	}
	// SEND
	if len(args) >= 2 && args[0] == "send" {
		if args[len(args)-1] == "-" {
			return []prompt.Suggest{{"-h", "Add custom header"}}
		}
		if args[len(args)-1] == "-h" || args[len(args)-2] == "-h" {
			return prompt.FilterFuzzy(headerSuggestions, d.GetWordBeforeCursorWithSpace(), true)
		}
	}
	// READ
	if len(args) >= 2 && args[0] == "read" {
		if args[len(args)-1] == "-" {
			return []prompt.Suggest{{"-a", "Read ALL messages"}}
		}
	}
	return []prompt.Suggest{}
}

func main() {

	prompt.New(
		executor,
		completion,
		prompt.OptionPrefix("stomper ⋙ "),
		prompt.OptionLivePrefix(livePrefix),
		prompt.OptionTitle("🥾 stomper"),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
		prompt.OptionSuggestionBGColor(prompt.Fuchsia),
		prompt.OptionDescriptionBGColor(prompt.Green),
		prompt.OptionSelectedSuggestionBGColor(prompt.Green),
		prompt.OptionSelectedDescriptionBGColor(prompt.Fuchsia),
		prompt.OptionScrollbarBGColor(prompt.LightGray),
		prompt.OptionPrefixTextColor(prompt.Purple),
		prompt.OptionCompletionOnDown(),
	).Run()
}
