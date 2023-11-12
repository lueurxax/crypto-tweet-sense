package keys

type Builder interface {
	Version() []byte
	Tweets() []byte
	Tweet(id string) []byte
}

type builder struct {
}

func (b builder) Version() []byte {
	return []byte{version}
}

func (b builder) Tweets() []byte {
	return []byte{tweet}
}

func (b builder) Tweet(id string) []byte {
	return append([]byte{tweet}, []byte(id)...)
}

func NewBuilder() Builder {
	return &builder{}
}
