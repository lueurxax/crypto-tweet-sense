package tweets_manager

type Manager interface {
	Start() error
}

type manager struct {
}
