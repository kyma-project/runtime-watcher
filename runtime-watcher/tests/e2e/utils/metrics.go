package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"
)

func ExposeSKRMetricsServiceEndpoint() error {
	cmd := exec.Command("kubectl", "config", "use-context", "k3d-skr")
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to switch context %w", err)
	}

	cmd = exec.Command("kubectl", "patch", "svc", "skr-webhook-metrics", "-p",
		"{\"spec\": {\"type\": \"LoadBalancer\"}}", "-n", "kyma-system")
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to patch metrics service %w", err)
	}

	return nil
}

func GetWatcherRequestDurationMetric(ctx context.Context) (float64, error) {
	metricsBody, err := getMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	regex := regexp.MustCompile(`watcher_request_duration ([0-9]*\.?[0-9]+)`)

	match := regex.FindStringSubmatch(metricsBody)
	if len(match) < 1 {
		return 0, fmt.Errorf("metric %s not found", watchermetrics.RequestDuration)
	}

	duration, err := strconv.ParseFloat(match[1], 64)
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
