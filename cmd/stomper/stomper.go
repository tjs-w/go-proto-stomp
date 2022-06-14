package main

import (
	"flag"
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
	transport = "tcp"
)

func errorMsg(s string) {
	_, _ = errPrint.Println(s)
}

func okMsg(s string) {
	_, _ = okPrint.Println(s)
}

var suggestions = []prompt.Suggest{
	// STOMP Commands
	{Text: "connect", Description: "Connect to a STOMP broker"},
	{Text: "disconnect", Description: "Disconnect from the STOMP broker"},
	{Text: "subscribe", Description: "Subscribe to a destination on broker"},
	{Text: "unsubscribe", Description: "Unsubscribe from a destination on broker"},
	{Text: "send", Description: "Send a message to a destination on broker"},
	{Text: "tx_leave", Description: "Leave an ongoing transaction"},
	{Text: "tx_enter", Description: "Enter an ongoing transaction"},
	{Text: "tx_begin", Description: "Start a transaction"},
	{Text: "tx_abort", Description: "Abort the transaction"},
	{Text: "tx_commit", Description: "Commit the transaction"},
	{Text: "session", Description: "Show session info"},
	{Text: "read", Description: "Read received messages"},
	{Text: "help", Description: "Display commands help"},
	{Text: "bye", Description: "End session by disconnecting from the STOMP broker"},
	{Text: "quit", Description: "End session by disconnecting from the STOMP broker"},
}

var headerSuggestions = []prompt.Suggest{
	// HTTP Header
	{Text: "Accept", Description: "Acceptable response media type"},
	{Text: "Accept-Charset", Description: "Acceptable response charsets"},
	{Text: "Accept-Encoding", Description: "Acceptable response content codings"},
	{Text: "Accept-Language", Description: "Preferred natural languages in response"},
	{Text: "ALPN", Description: "Application-layer protocol negotiation to use"},
	{Text: "Alt-Used", Description: "Alternative host in use"},
	{Text: "Authorization", Description: "Authentication information"},
	{Text: "Cache-Control", Description: "Directives for caches"},
	{Text: "Connection", Description: "Connection options"},
	{Text: "Content-Encoding", Description: "Content codings"},
	{Text: "Content-Language", Description: "Natural languages for content"},
	{Text: "Content-Length", Description: "Anticipated size for payload body"},
	{Text: "Content-Location", Description: "Where content was obtained"},
	{Text: "Content-MD5", Description: "Base64-encoded MD5 sum of content"},
	{Text: "Content-Type", Description: "Content media type"},
	{Text: "Cookie", Description: "Stored cookies"},
	{Text: "Date", Description: "Datetime when message was originated"},
	{Text: "Depth", Description: "Applied only to resource or its members"},
	{Text: "DNT", Description: "Do not track user"},
	{Text: "Expect", Description: "Expected behaviors supported by server"},
	{Text: "Forwarded", Description: "Proxies involved"},
	{Text: "From", Description: "Sender email address"},
	{Text: "VirtualHost", Description: "Target URI"},
	{Text: "HTTP2-Settings", Description: "HTTP/2 connection parameters"},
	{Text: "If", Description: "Request condition on state tokens and ETags"},
	{Text: "If-Match", Description: "Request condition on target resource"},
	{Text: "If-Modified-Since", Description: "Request condition on modification date"},
	{Text: "If-None-Match", Description: "Request condition on target resource"},
	{Text: "If-Range", Description: "Request condition on Range"},
	{Text: "If-Schedule-Tag-Match", Description: "Request condition on Schedule-Tag"},
	{Text: "If-Unmodified-Since", Description: "Request condition on modification date"},
	{Text: "Max-Forwards", Description: "Max number of times forwarded by proxies"},
	{Text: "MIME-Version", Description: "Version of MIME protocol"},
	{Text: "Origin", Description: "Origin(s} issuing the request"},
	{Text: "Pragma", Description: "Implementation-specific directives"},
	{Text: "Prefer", Description: "Preferred server behaviors"},
	{Text: "Proxy-Authorization", Description: "Proxy authorization credentials"},
	{Text: "Proxy-Connection", Description: "Proxy connection options"},
	{Text: "Range", Description: "Request transfer of only part of data"},
	{Text: "Referer", Description: "Previous web page"},
	{Text: "TE", Description: "Transfer codings willing to accept"},
	{Text: "Transfer-Encoding", Description: "Transfer codings applied to payload body"},
	{Text: "Upgrade", Description: "Invite server to upgrade to another protocol"},
	{Text: "User-Agent", Description: "User agent string"},
	{Text: "Via", Description: "Intermediate proxies"},
	{Text: "Warning", Description: "Possible incorrectness with payload body"},
	{Text: "WWW-Authenticate", Description: "Authentication scheme"},
	{Text: "X-Csrf-Token", Description: "Prevent cross-site request forgery"},
	{Text: "X-CSRFToken", Description: "Prevent cross-site request forgery"},
	{Text: "X-Forwarded-For", Description: "Originating client IP address"},
	{Text: "X-Forwarded-VirtualHost", Description: "Original host requested by client"},
	{Text: "X-Forwarded-Proto", Description: "Originating protocol"},
	{Text: "X-Http-Method-Override", Description: "Request method override"},
	{Text: "X-Requested-With", Description: "Used to identify Ajax requests"},
	{Text: "X-XSRF-TOKEN", Description: "Prevent cross-site request forgery"},
}

func setupConnection(host string, port string, t stomp.Transport) {
	keyPrint := color.New(color.FgRed, color.Bold, color.Italic).SprintFunc()
	valPrint := color.New(color.FgYellow).SprintFunc()
	bodyPrint := func(b []byte) []byte {
		return []byte(color.New(color.FgBlue).Sprint(string(b)))
	}

	msgHandler := func(message *stomp.UserMessage) {
		sb := strings.Builder{}
		for k, v := range message.Headers {
			sb.WriteString(fmt.Sprintf("%s:%s\n", keyPrint(k), valPrint(v)))
		}
		sb.Write(bodyPrint(message.Body))
		ctx.messages = append(ctx.messages, sb.String())
	}

	ctx.client = stomp.NewClientHandler(t, host, port, &stomp.ClientOpts{
		VirtualHost:    host,
		MessageHandler: msgHandler,
	})
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
	t := stomp.TransportTCP
	if transport == "websocket" {
		t = stomp.TransportWebsocket
	}
	setupConnection(in[1], in[2], t)
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
			return []prompt.Suggest{{Text: "-h", Description: "Add custom header"}}
		}
		if args[len(args)-1] == "-h" || args[len(args)-2] == "-h" {
			return prompt.FilterFuzzy(headerSuggestions, d.GetWordBeforeCursorWithSpace(), true)
		}
	}
	// READ
	if len(args) >= 2 && args[0] == "read" {
		if args[len(args)-1] == "-" {
			return []prompt.Suggest{{Text: "-a", Description: "Read ALL messages"}}
		}
	}
	return []prompt.Suggest{}
}

func main() {
	flag.StringVar(&transport, "t", "websocket", "transport for STOMP protocol (tcp, websocket)")
	flag.Parse()

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
