package chtun

import (
	"log"
	"net"
	"runtime"
	"strconv"
)

// setRoute sets the system routes
func setRoute(iface string, physicalIface string, localGateway string, serverAddrIP string) {
	CIDR := "172.18.0.1/32"
	ServerIP := "172.18.0.1"
	MTU := 1500

	ip, _, err := net.ParseCIDR(CIDR)
	if err != nil {
		log.Panicf("error cidr %v", CIDR)
	}
	os := runtime.GOOS
	if os == "linux" {
		ExecCmd("/sbin/ip", "link", "set", "dev", iface, "mtu", strconv.Itoa(MTU))
		ExecCmd("/sbin/ip", "addr", "add", CIDR, "dev", iface)
		ExecCmd("/sbin/ip", "link", "set", "dev", iface, "up")

		ExecCmd("/sbin/ip", "route", "add", serverAddrIP+"/32", "via", localGateway, "dev", physicalIface)
		ExecCmd("/sbin/ip", "route", "add", "0.0.0.0/1", "dev", iface)
		ExecCmd("/sbin/ip", "route", "add", "128.0.0.0/1", "dev", iface)

	} else if os == "darwin" {
		ExecCmd("ifconfig", iface, "inet", ip.String(), ServerIP, "up")

		ExecCmd("route", "add", serverAddrIP, localGateway)
		ExecCmd("route", "add", "default", ServerIP)
		ExecCmd("route", "change", "default", ServerIP)
		ExecCmd("route", "add", "0.0.0.0/1", "-interface", iface)
		ExecCmd("route", "add", "128.0.0.0/1", "-interface", iface)

	} else if os == "windows" {

		ExecCmd("cmd", "/C", "route", "add", serverAddrIP+"/32", localGateway, "metric", "5")
		ExecCmd("cmd", "/C", "route", "delete", "0.0.0.0", "mask", "0.0.0.0")
		ExecCmd("cmd", "/C", "route", "add", "0.0.0.0", "mask", "0.0.0.0", ServerIP, "metric", "6")

	} else {
		log.Printf("not support os %v", os)
	}
	log.Printf("interface configured %v", iface)
}

// ResetRoute resets the system routes
func resetRoute(iface string, physicalIface string, localGateway string, serverAddrIP string) {

	os := runtime.GOOS
	if os == "linux" {
		ExecCmd("/sbin/ip", "route", "delete", serverAddrIP+"/32")
		ExecCmd("/sbin/ip", "link", "delete", iface)
	} else if os == "darwin" {
		ExecCmd("route", "add", "default", localGateway)
		ExecCmd("route", "change", "default", localGateway)
		ExecCmd("route", "delete", serverAddrIP)
	} else if os == "windows" {
		ExecCmd("cmd", "/C", "route", "delete", serverAddrIP+"/32")
		ExecCmd("cmd", "/C", "route", "delete", "0.0.0.0", "mask", "0.0.0.0")
		ExecCmd("cmd", "/C", "route", "add", "0.0.0.0", "mask", "0.0.0.0", localGateway, "metric", "6")
	}
}
