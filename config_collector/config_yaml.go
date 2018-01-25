package config_collector

import (
	"io/ioutil"
	"fmt"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"strings"
	"net"
	"os/exec"
	"net/http"
	"net/url"
	"strconv"
	"crypto/tls"
)

const (
	ConfigPath = "/etc/node_exporter/yml"
	network  = "tcp"
	network4 = "tcp4"
	minPort  = 0
	MaxPort  = 65535
)

type LabelName string
type LabelValue string
type Labels map[LabelName]LabelValue

// 定义一个结构体,配置config.yaml
type Config struct {
	Kind     string    `yaml:"kind"`
	Metadata *Metadata `yaml:"metadata"`
	Spec     *Spec     `yaml:"spec"`
	// 从该配置被解析的输入内容
	original string
}

type Metadata struct {
	Labels *Labels `yaml:"labels"`
	Metric string  `yaml:"metric"`
	Help   string  `yaml:"help"`
}

type Spec struct {
	ALive *IsLive `yaml:"alive"`
}

type IsLive struct {
	Tcp          *Tcp   `yaml:"tcp"`
	Http         *Http  `yaml:"http"`
	Exec         *Exec  `yaml:"exec"`
	DelaySeconds string `yaml:"delayseconds"`
}

type Tcp struct {
	Port int `yaml:"port"`
}

type Http struct {
	Scheme string `yaml:"scheme,omitempty"`
	Host   string `yaml:"host,omitempty"`
	Port   int    `yaml:"port"`
	Path   string `yaml:"path"`
	Method string `yaml:"method"`
}

type Exec struct {
	Command []string `yaml:"command"`
}

func (ls *IsLive) Validate(key string) bool {
	if ls == nil {
		return false
	} else if strings.ToUpper(key) == "TCP" {
		return ls.Tcp != nil
	} else if strings.ToUpper(key) == "HTTP" {
		return ls.Http != nil
	} else if strings.ToUpper(key) == "EXEC" {
		return ls.Exec != nil
	} else {
		return false
	}
}

// Load parses the YAML input s into a Config.
func Load(s string) (*Config, error) {
	cfg := &Config{}
	err := yaml.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	cfg.original = s
	return cfg, nil
}

// LoadFile parses the given YAML file into a Config.
func LoadFile(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg, err := Load(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing YAML file %s: %v", filename, err)
	}
	return cfg, nil
}

// LoadFiles
func LoadFolderFiles(folderPath string) ([]string, error) {
	var results []string
	if fileList, err := filepath.Glob(
		strings.TrimSuffix(folderPath, "/") + "/*.yml"); err != nil {
		return results, nil
	} else {
		for _, filename := range fileList {
			results = append(results, filename)
		}
	}
	return results, nil

}

func (c *Config) MetricInfo(value int) string {
	formatData := `# HELP %s
# TYPE %s gauge
%s %d
`
	labels := ""
	for k, v := range *c.Metadata.Labels {
		labels = labels + fmt.Sprintf("%s=\"%s\", ", k, v)
	}
	if len(labels) != 0 {
		labels = fmt.Sprintf("%s{%s}", c.Metadata.Metric, strings.TrimSuffix(labels, ", "))
	} else {
		labels = c.Metadata.Metric
	}
	formatData = fmt.Sprintf(formatData,
		c.Metadata.Metric+" "+c.Metadata.Help, c.Metadata.Metric, labels, value)
	return formatData
}

// 健康检查
func (t *Tcp) Healthy() int {
	if t.Port <= minPort || t.Port >= MaxPort {
		return 0
	}
	addr := fmt.Sprintf("0.0.0.0:%d", t.Port)
	tcpAddr, err := net.ResolveTCPAddr(network4, addr)
	if err != nil {
		return 0
	}
	conn, err := net.DialTCP(network, nil, tcpAddr)
	if err != nil {
		return 0
	}
	defer conn.Close()
	return 1
}

// 健康检查
func (t *Http) Healthy() int {
	if t.Port <= minPort || t.Port >= MaxPort {
		return 0
	}
	urlPath := FormatURL(t.Scheme, t.Host, t.Port, t.Path).String()
	verify := false
	if strings.HasPrefix(strings.ToLower(urlPath), "https") {
		verify = true
	}
	if t.Method == "GET" {
		return Get(urlPath, verify)
	} else if t.Method == "POST" {
		return Post(urlPath, verify)
	} else if t.Method == "HEAD" {
		return Head(urlPath, verify)
	} else {
		return 0
	}
}

// 健康检查
func (t *Exec) Healthy() int {
	if len(t.Command) <= 0 {
		return 0
	}
	cmd := exec.Command(t.Command[0], t.Command[1:]...)
	if _, err := cmd.CombinedOutput(); err != nil {
		return 0
	} else {
		return 1
	}
}

// get http client
func getHttpClient(verify bool) *http.Client {
	client := &http.Client{}
	if verify {
		tlsVerify := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		client = &http.Client{Transport: tlsVerify}
	}
	return client
}

// common response
func response(request *http.Request, err error, verify bool) int {
	if err != nil {
		return 0
	}
	client := getHttpClient(verify)
	resp, e := client.Do(request)
	if e != nil {
		return 0
	}
	defer resp.Body.Close()
	_, e = ioutil.ReadAll(resp.Body)
	if e != nil {
		return 0
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return 1
	}
	return 0
}

// get
func Get(url string, verify bool) int {
	request, e := http.NewRequest(http.MethodGet, url, strings.NewReader(""))
	return response(request, e, verify)
}

// post
func Post(url string, verify bool) int {
	request, e := http.NewRequest(http.MethodPost, url, strings.NewReader(""))
	return response(request, e, verify)
}

// head
func Head(url string, verify bool) int {
	request, e := http.NewRequest(http.MethodHead, url, strings.NewReader(""))
	return response(request, e, verify)
}

// 格式化url
func FormatURL(scheme string, host string, port int, path string) *url.URL {
	u, err := url.Parse(path)
	if err != nil {
		u = &url.URL{
			Path: path,
		}
	}
	u.Scheme = scheme
	u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	return u

}
