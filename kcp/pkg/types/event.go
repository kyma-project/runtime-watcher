package types

type WatcherEvent struct {
	KymaCr      string       `json:"kyma"`
	Namespace   string       `json:"namespace"`
	Name        string       `json:"name"`
	KymaModules []KymaModule `json:"modules"`
}

type KymaModule struct {
	Name    string `json:"name"`
	Channel string `json:"channel"`
}
