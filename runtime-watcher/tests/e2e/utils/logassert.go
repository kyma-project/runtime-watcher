package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogAsserter struct {
	controlPlaneConfig *rest.Config
	runtimeConfig      *rest.Config
	controlPlaneClient client.Client
	runtimeClient      client.Client
}

func NewLogAsserter(
	controlPlaneConfig,
	runtimeConfig *rest.Config,
	controlPlaneClient,
	runtimeClient client.Client,
) *LogAsserter {
	return &LogAsserter{
		controlPlaneConfig: controlPlaneConfig,
		runtimeConfig:      runtimeConfig,
		controlPlaneClient: controlPlaneClient,
		runtimeClient:      runtimeClient,
	}
}

const (
	controlPlaneNamespace = "kcp-system"
	remoteNamespace       = "kyma-system"
	klmPodPrefix          = "klm-controller-manager"
	klmPodContainer       = "manager"
	watcherPodPrefix      = "skr-webhook"
	watcherPodContainer   = "server"
)

var (
	errPodNotFound         = errors.New("could not find pod")
	errKLMLogMsgNotFound   = errors.New("log msg was not found in KLM log")
	errWatcherLogsNotFound = errors.New("watcher log was not found since")
)

func (l *LogAsserter) ContainsKLMLogMessage(ctx context.Context, msg string, since *apimetav1.Time) error {
	logs, err := fetchLogsFromPod(
		ctx,
		l.controlPlaneConfig,
		l.controlPlaneClient,
		controlPlaneNamespace,
		klmPodPrefix,
		klmPodContainer,
		since)
	if err != nil {
		return err
	}

	if strings.Contains(logs, msg) {
		return nil
	}

	return errKLMLogMsgNotFound
}

func (l *LogAsserter) ContainsWatcherLogs(ctx context.Context, since *apimetav1.Time) error {
	_, err := fetchLogsFromPod(
		ctx,
		l.runtimeConfig,
		l.runtimeClient,
		remoteNamespace,
		watcherPodPrefix,
		watcherPodContainer,
		since)
	if err != nil {
		return errors.Join(err, errWatcherLogsNotFound)
	}
	return nil
}

func fetchLogsFromPod(ctx context.Context,
	config *rest.Config,
	clnt client.Client,
	namespace, podPrefix, container string,
	since *apimetav1.Time,
) (string, error) {
	pod := apicorev1.Pod{}
	podList := &apicorev1.PodList{}
	if err := clnt.List(ctx, podList, &client.ListOptions{Namespace: namespace}); err != nil {
		return "", fmt.Errorf("failed to list pods %w", err)
	}

	for i := range podList.Items {
		pod = podList.Items[i]
		if strings.HasPrefix(pod.Name, podPrefix) {
			break
		}
	}
	if pod.Name == "" {
		return "", fmt.Errorf("%w: Prefix: %s Container: %s", errPodNotFound, podPrefix, container)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create clientset, %w", err)
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &apicorev1.PodLogOptions{
		Container: container,
		SinceTime: since,
	})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs %w", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to copy pod logs %w", err)
	}
	str := buf.String()

	return str, nil
}
