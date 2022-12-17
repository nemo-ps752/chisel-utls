package chtun

import (
	"log"
	"net"
	"runtime"

	netroute "github.com/libp2p/go-netroute"
	"github.com/xjasonlyu/tun2socks/v2/engine"
	"go.uber.org/automaxprocs/maxprocs"
)

func RunTun2Socks(serverIP string, wait chan int, done chan int) {
	router, _ := netroute.New()
	iface, gw, src, err := router.Route(net.IPv4(8, 8, 8, 8))
	log.Println(iface.Name, gw.String(), src.String(), err)

	key := new(engine.Key)

	key.Mark = 0
	key.MTU = 1500
	key.UDPTimeout = 0
	os := runtime.GOOS
	if os == "windows" {
		key.Device = "wintun"
	} else {
		key.Device = "utun888"
	}
	key.Interface = iface.Name
	key.LogLevel = "silent"
	key.Proxy = "socks5://127.0.0.1:9050"
	key.RestAPI = ""
	key.TCPReceiveBufferSize = ""
	key.TCPSendBufferSize = ""
	key.TCPModerateReceiveBuffer = false

	maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))
	engine.Insert(key)

	engine.Start()

	log.Println("Engine Ready!")
	setRoute(key.Device, iface.Name, gw.String(), serverIP)
	<-wait
	log.Println("Stopping tun2socks...")
	engine.Stop()
	resetRoute(key.Device, iface.Name, gw.String(), serverIP)
	done <- 0
}
