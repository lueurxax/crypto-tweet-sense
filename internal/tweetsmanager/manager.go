package tweetsmanager

type Manager interface {
	Start() error
}

type manager struct {
}
