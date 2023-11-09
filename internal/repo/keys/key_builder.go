package keys

type Builder interface {
	Version() []byte
	Tweets() []byte
}

type builder struct {
}

func (b builder) Version() []byte {
	//TODO implement me
	panic("implement me")
}

func (b builder) Tweets() []byte {
	//TODO implement me
	panic("implement me")
}

func NewBuilder() Builder {
	return &builder{}
}
