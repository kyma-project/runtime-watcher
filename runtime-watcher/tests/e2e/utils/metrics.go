package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	exec2 "os/exec"
	"regexp"
	"strconv"

	"github.com/onsi/ginkgo/v2/dsl/core"

	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"
)

func ExposeSKRMetricsEndpoint() error {
	cmd := exec2.Command("kubectl", "config", "use-context", "k3d-skr")
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}

	cmd = exec2.Command("kubectl", "patch", "svc", "skr-webhook-metrics", "-p",
		"[{\"op\": \"replace\", \"path\": \"/spec/ports/0/nodePort\", \"value\":2112}]",
		"--type=json", "-n", "kyma-system")
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}

	return nil
}

func GetWatcherRequestDurationMetric(ctx context.Context) (float64, error) {
	metricsBody, err := getMetricsBody(ctx)
	core.GinkgoWriter.Println(metricsBody)

	if err != nil {
		return 0, err
	}

	regex := regexp.MustCompile(watchermetrics.RequestDuration)
	core.GinkgoWriter.Println(regex)

	match := regex.FindStringSubmatch(metricsBody)
	if len(match) < 1 {
		core.GinkgoWriter.Println("METRIC NOT FOUND")

		return 0, fmt.Errorf("metric %s not found", watchermetrics.RequestDuration)
	}

	duration, err := strconv.ParseFloat(match[1], 64)
	core.GinkgoWriter.Println("------------------")

	core.GinkgoWriter.Println(duration)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse metric %s value", watchermetrics.RequestDuration)
	}
	return duration, nil
}

func getMetricsBody(ctx context.Context) (string, error) {
	clnt := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:2112/metrics", nil)
	if err != nil {
		return "", fmt.Errorf("request to metrics endpoint :%w", err)
	}
	response, err := clnt.Do(request)
	if err != nil {
		return "", fmt.Errorf("response from metrics endpoint :%w", err)
	}
	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("response body:%w", err)
	}
	bodyString := string(bodyBytes)

	return bodyString, nil
}
