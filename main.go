package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	http "github.com/ooni/oohttp"

	chclient "utunnel/client"
	chserver "utunnel/server"
	"utunnel/share/cos"
	chtun "utunnel/tun"

	socks5 "github.com/txthinking/socks5"

	_ "github.com/xjasonlyu/tun2socks/v2/dns"
)

func main() {

	flag.Bool("help", false, "")
	flag.Bool("h", false, "")
	flag.Usage = func() {}
	flag.Parse()

	args := flag.Args()

	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "server":
		server(args)
	case "client":
		client(args)
	default:
		os.Exit(0)
	}
}

func generatePidFile() {
	pid := []byte(strconv.Itoa(os.Getpid()))
	if err := ioutil.WriteFile("utunnel.pid", pid, 0644); err != nil {
		log.Fatal(err)
	}
}

func server(args []string) {

	flags := flag.NewFlagSet("server", flag.ContinueOnError)

	config := &chserver.Config{}
	flags.StringVar(&config.KeySeed, "key", "", "")
	flags.StringVar(&config.AuthFile, "authfile", "", "")
	flags.StringVar(&config.Auth, "auth", "", "")
	flags.DurationVar(&config.KeepAlive, "keepalive", 25*time.Second, "")
	flags.StringVar(&config.Proxy, "proxy", "", "")
	flags.StringVar(&config.Proxy, "backend", "", "")
	flags.BoolVar(&config.Socks5, "socks5", false, "")
	flags.BoolVar(&config.Reverse, "reverse", false, "")
	flags.StringVar(&config.TLS.Key, "tls-key", "", "")
	flags.StringVar(&config.TLS.Cert, "tls-cert", "", "")
	flags.StringVar(&config.TLS.CA, "tls-ca", "", "")

	host := flags.String("host", "", "")
	p := flags.String("p", "", "")
	port := flags.String("port", "", "")
	pid := flags.Bool("pid", false, "")
	verbose := flags.Bool("v", false, "")

	flags.Parse(args)

	if *host == "" {
		*host = os.Getenv("HOST")
	}
	if *host == "" {
		*host = "0.0.0.0"
	}
	if *port == "" {
		*port = *p
	}
	if *port == "" {
		*port = os.Getenv("PORT")
	}
	if *port == "" {
		*port = "8080"
	}
	if config.KeySeed == "" {
		config.KeySeed = os.Getenv("utunnel_KEY")
	}
	s, err := chserver.NewServer(config)
	if err != nil {
		log.Fatal(err)
	}
	s.Debug = *verbose
	if *pid {
		generatePidFile()
	}
	ctx := cos.InterruptContext()
	if err := s.StartContext(ctx, *host, *port); err != nil {
		log.Fatal(err)
	}
	done := make(chan int)
	wait := make(chan int)
	go socksServer(done, wait)

	if err := s.Wait(); err != nil {
		log.Fatal(err)
	}
	done <- 0
	<-wait
}

func socksServer(done chan int, wait chan int) {

	s5, err := socks5.NewClassicServer("127.0.0.1:9050", "127.0.0.1", "", "", 30, 30)
	if err != nil {
		log.Fatal(err)
	}
	go s5.ListenAndServe(s5.Handle)
	<-done
	s5.Shutdown()
	log.Println("Closed Socks Server!")
	wait <- 0

}

type headerFlags struct {
	http.Header
}

func (flag *headerFlags) String() string {
	out := ""
	for k, v := range flag.Header {
		out += fmt.Sprintf("%s: %s\n", k, v)
	}
	return out
}

func (flag *headerFlags) Set(arg string) error {
	index := strings.Index(arg, ":")
	if index < 0 {
		return fmt.Errorf(`invalid header (%s). should be in the format "HeaderName: HeaderContent"`, arg)
	}
	if flag.Header == nil {
		flag.Header = http.Header{}
	}
	key := arg[0:index]
	value := arg[index+1:]
	flag.Header.Set(key, strings.TrimSpace(value))
	return nil
}

func client(args []string) {
	flags := flag.NewFlagSet("client", flag.ContinueOnError)
	config := chclient.Config{Headers: http.Header{}}
	flags.StringVar(&config.Fingerprint, "fingerprint", "", "")
	flags.StringVar(&config.Auth, "auth", "", "")
	flags.DurationVar(&config.KeepAlive, "keepalive", 25*time.Second, "")
	flags.IntVar(&config.MaxRetryCount, "max-retry-count", -1, "")
	flags.DurationVar(&config.MaxRetryInterval, "max-retry-interval", 0, "")
	flags.StringVar(&config.Proxy, "proxy", "", "")
	flags.StringVar(&config.TLS.CA, "tls-ca", "", "")
	flags.BoolVar(&config.TLS.SkipVerify, "tls-skip-verify", false, "")
	flags.StringVar(&config.TLS.Cert, "tls-cert", "", "")
	flags.StringVar(&config.TLS.Key, "tls-key", "", "")
	flags.Var(&headerFlags{config.Headers}, "header", "")
	sni := flags.String("sni", "", "")
	pid := flags.Bool("pid", false, "")
	verbose := flags.Bool("v", false, "")
	flags.Parse(args)
	//pull out options, put back remaining args
	config.Server = flags.Args()[0]

	config.Remotes = flags.Args()[1:]
	config.Remotes = append(config.Remotes, "9050:127.0.0.1:9050")
	config.Remotes = append(config.Remotes, "9050:127.0.0.1:9050/udp")
	//default auth
	if config.Auth == "" {
		config.Auth = os.Getenv("AUTH")
	}
	//move hostname onto headers
	parsedUrl, err := url.Parse(config.Server)
	if err != nil {
		log.Fatal(err)
	}
	addrs, err := net.LookupHost(parsedUrl.Hostname())
	if err != nil {
		log.Fatal(err)
	}
	config.Headers.Set("Host", parsedUrl.Hostname())
	config.TLS.ServerName = parsedUrl.Hostname()

	if *sni != "" {
		config.TLS.ServerName = *sni
	}
	log.Printf("Connection Address: %v", addrs[0])

	//ready
	c, err := chclient.NewClient(&config)
	if err != nil {
		log.Fatal(err)
	}
	c.Debug = *verbose
	if *pid {
		generatePidFile()
	}
	ctx := cos.InterruptContext()
	ready := make(chan int)
	if err := c.Start(ready, ctx); err != nil {
		log.Fatal(err)
	}
	wait := make(chan int)
	done := make(chan int)
	go chtun.RunTun2Socks(addrs[0], ready, wait, done)
	if err := c.Wait(); err != nil {
		wait <- 0
		<-done
		log.Fatal(err)
	}
	wait <- 0
	<-done
}
