package keys

type Builder interface {
	Version() []byte
	Tweets() []byte
}

type builder struct {
}

func (b builder) Version() []byte {
	return []byte{version}
}

func (b builder) Tweets() []byte {
	return []byte{tweet}
}

func NewBuilder() Builder {
	return &builder{}
}
