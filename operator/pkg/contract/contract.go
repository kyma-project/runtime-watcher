package contract

type WatcherEvent struct {
	KymaCr    string `json:"kyma"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}
