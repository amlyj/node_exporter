package config_collector

import (
	"strings"
	"github.com/prometheus/common/log"
	"errors"
)

func GetConfigCollectorMetrics(s string) string {
	metricList := []string{s}
	if fileList, err := LoadFolderFiles(ConfigPath); err != nil {
		log.Errorln("getConfigCollectorMetrics error: ", err)
	} else {
		for _, filename := range fileList {
			data, err := YAMLMetric(filename)
			if err != nil {
				continue
			}
			metricList = append(metricList, data)
		}
	}
	return metricJoin(metricList...)
}

func metricJoin(s ... string) string {
	var res string
	for _, v := range s {
		if len(res) == 0 || strings.HasSuffix(res, "\n") {
			res = res + v
		} else {
			res = res + "\n" + v
		}
	}
	return res
}

func YAMLMetric(filePath string) (string, error) {
	config, err := LoadFile(filePath)
	if err != nil {
		return "", err
	}
	alive := config.Spec.ALive
	if alive == nil {
		return "", errors.New(filePath + " parser yaml error.")
	}
	if alive.Validate("tcp") {
		return config.MetricInfo(alive.Tcp.Healthy()), nil
	} else if alive.Validate("exec") {
		return config.MetricInfo(alive.Exec.Healthy()), nil
	} else if alive.Validate("http") {
		return config.MetricInfo(alive.Http.Healthy()), nil
	} else {
		return "", nil
	}

}
