package collector

import (
	"net/http"
	"time"
	"github.com/prometheus/common/log"
	"net"
	"fmt"
	"errors"
	"io/ioutil"
	"strings"
)

func PushMetrics(listenAddress string, metricsPath string, pushGateway string, jobName string) {
	go startPush(listenAddress, metricsPath, pushGateway, jobName)
	http.HandleFunc("/push", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>push worker</title></head>
			<body><h1>push worker</h1></body>
			</html>`))
	})
	log.Infoln("pushGateway task Listening on", ":9901")
	err := http.ListenAndServe(":9901", nil)
	if err != nil {
		log.Fatal("start error:")
		log.Fatal(err)
	}
}

// loop push
func startPush(listenAddress string, metricsPath string, pushGateway string, jobName string) {
	for {
		time.Sleep(time.Second * 15)
		log.Infoln("push metrics...")
		Try(func() {
			pushGateWay(listenAddress, metricsPath, pushGateway, jobName)
		}, func(e interface{}) {
			fmt.Printf("push error: %s", e)
		})
	}
}

func pushGateWay(listenAddress string, metricsPath string, pushGateway string, jobName string) {
	ip, err := getInterface()
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	requestUrl := fmt.Sprintf("http://localhost%s%s", listenAddress, metricsPath)
	postUrl := fmt.Sprintf("http://%s/metrics/job/%s/instance/%s%s", pushGateway, jobName, ip, listenAddress)

	r, err := http.Get(requestUrl)
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	data := string(body)
	resp, err := http.Post(postUrl, "multipart/form-data", strings.NewReader(data))
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	log.Infoln("success!", time.Now())
}

func Try(fun func(), handler func(interface{})) {
	defer func() {
		if err := recover(); err != nil {
			handler(err)
		}
	}()
	fun()
}

func getInterface() (string, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	for _, address := range addresses {
		if ipInfo, ok := address.(*net.IPNet); ok &&
			!ipInfo.IP.IsLoopback() &&
			ipInfo.IP.IsGlobalUnicast() {
			if ipInfo.IP.To4() != nil {
				return ipInfo.IP.String(), nil
			}
		}
	}
	return "", errors.New("can not find 1 available ip address")
}
