package main

import (
	"context"
	"encoding/base64"
	"image/color"
	"io"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	http "github.com/ooni/oohttp"

	chclient "utunnel/client"
	chshare "utunnel/share"
	"utunnel/share/cos"
	chtun "utunnel/tun"

	"github.com/getlantern/elevate"
	viper "github.com/spf13/viper"
)

var APP_NAME = "uTunnel"

type LogWriter struct {
	io.Writer
}

var chnLogger = make(chan string)
var chnStatus = make(chan chclient.ConnectionStatusEnum)

func (l LogWriter) Write(p []byte) (int, error) {
	chnLogger <- string(p)
	return len(p), nil
}

func TouchFile(name string) error {
	file, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

var client *chclient.Client

func Connect(server string, username string, password string) {
	config := chclient.Config{Headers: http.Header{}}
	config.Fingerprint = ""
	config.Auth = username + ":" + password
	config.KeepAlive = 25 * time.Second
	config.MaxRetryCount = -1
	config.MaxRetryInterval = 0
	config.Proxy = ""
	config.TLS.CA = ""
	config.TLS.SkipVerify = false
	config.TLS.Cert = ""
	config.TLS.Key = ""

	//pull out options, put back remaining args
	config.Server = server

	config.Remotes = make([]string, 0)
	config.Remotes = append(config.Remotes, "9050:127.0.0.1:9050")
	config.Remotes = append(config.Remotes, "9050:127.0.0.1:9050/udp")

	parsedUrl, err := url.Parse(config.Server)
	if err != nil {
		log.Println(err)
	}
	addrs, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", parsedUrl.Hostname())
	if err != nil {
		log.Println(err)
	}
	config.Headers.Set("Host", parsedUrl.Hostname())
	config.TLS.ServerName = parsedUrl.Hostname()

	ipAddress := addrs[rand.Intn(len(addrs))].String()
	log.Printf("Connection Address: %v", ipAddress)
	config.Server = "https://" + ipAddress + "/data"

	client, err = chclient.NewClient(&config)
	client.Logger.SetOutput(LogWriter{})
	if err != nil {
		log.Println(err)
	}
	client.Debug = false
	ctx := cos.InterruptContext()
	if err := client.Start(ctx); err != nil {
		log.Println(err)
	}
	wait := make(chan int)
	done := make(chan int)
	isFirstTime := true
	client.ConnectionStatus.OnStatusChange = func(status chclient.ConnectionStatusEnum) {
		chnStatus <- status
		if status == chclient.Connected && isFirstTime {
			go chtun.RunTun2Socks(ipAddress, wait, done)
			isFirstTime = false
		}
	}
	err = client.Wait()
	if err != nil {
		log.Println(err)
	}
	wait <- 0
	<-done
}

func Disconnect() {
	if client != nil {
		client.Close()
	}
}

func IsAdmin() bool {
	goos := runtime.GOOS
	if goos == "windows" {
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		return err == nil
	} else {
		return os.Geteuid() == 0
	}
}

func main() {
	if !IsAdmin() {
		goos := runtime.GOOS
		if goos == "darwin" || goos == "windows" {
			cmd := elevate.Command(os.Args[0])
			cmd.Run()
		} else {
			dApp := app.New()
			window := dApp.NewWindow(APP_NAME)
			window.SetContent(
				container.NewBorder(
					widget.NewLabel("This app must run as administrator!"),
					widget.NewButton("OK", func() { os.Exit(0) }),
					nil, nil,
				),
			)
			window.ShowAndRun()
		}
		return
	}
	viper.SetConfigType("yaml")
	viper.SetConfigFile("./config.yml")
	viper.SetDefault("address", "")
	viper.SetDefault("username", "")
	viper.SetDefault("password", "")
	err := TouchFile("./config.yml")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	print(os.Geteuid())
	log.SetOutput(LogWriter{})
	appClient := app.New()
	appClient.SetIcon(chshare.LogoPNG)
	appClient.Settings().SetTheme(theme.DarkTheme())
	winWindow := appClient.NewWindow(APP_NAME)

	txtLogs := widget.NewMultiLineEntry()
	txtLogs.Wrapping = fyne.TextWrapBreak
	txtLogs.Disable()

	txtAddress := widget.NewEntry()
	txtAddress.Text = viper.GetString("address")
	txtAddress.OnChanged = func(s string) {
		viper.Set("address", s)
	}

	txtUsername := widget.NewEntry()
	txtUsername.Text = viper.GetString("username")
	txtUsername.OnChanged = func(s string) {
		viper.Set("username", s)
	}

	txtPassword := widget.NewPasswordEntry()
	password, _ := base64.StdEncoding.DecodeString(viper.GetString("password"))
	txtPassword.SetText(string(password))
	txtPassword.Password = true
	txtPassword.OnChanged = func(s string) {
		viper.Set("password", base64.StdEncoding.EncodeToString([]byte(s)))
	}

	form := widget.NewForm(
		widget.NewFormItem("Address", txtAddress),
		widget.NewFormItem("Username", txtUsername),
		widget.NewFormItem("Password", txtPassword),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Config", form),
		container.NewTabItem("Logs", txtLogs),
	)
	btnConnect := widget.NewButton("Connect", func() {})
	btnConnect.OnTapped = func() {
		viper.WriteConfig()
		if client != nil && client.ConnectionStatus.Status != chclient.Disconnected {
			go Disconnect()
		} else {
			go Connect(txtAddress.Text, txtUsername.Text, txtPassword.Text)
		}
	}
	btnBackground := canvas.NewRectangle(color.RGBA{R: 255})
	btnContainer := container.NewMax(btnConnect, btnBackground)

	go func() {
		for status := range chnStatus {
			println("Connection Status: ", status)
			if status == chclient.Connected {
				btnBackground.FillColor = color.RGBA{G: 255}
				btnBackground.Refresh()
				btnConnect.SetText("Disconnect")
			} else if status == chclient.Connecting {
				btnBackground.FillColor = color.RGBA{G: 255, R: 255}
				btnBackground.Refresh()
				btnConnect.SetText("Connecting...")
			} else {
				btnBackground.FillColor = color.RGBA{R: 255}
				btnBackground.Refresh()
				btnConnect.SetText("Connect")
			}
		}
	}()

	vbox := container.NewBorder(nil, btnContainer, nil, nil,
		tabs,
	)

	winWindow.Resize(fyne.NewSize(800, 450))
	winWindow.SetFixedSize(true)
	winWindow.SetContent(vbox)

	go func() {
		for text := range chnLogger {
			txtLogs.SetText(txtLogs.Text + text)
			txtLogs.CursorRow = strings.Count(txtLogs.Text, "\n")
		}
	}()

	winWindow.ShowAndRun()

}
