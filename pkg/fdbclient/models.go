package fdbclient

import "github.com/apple/foundationdb/bindings/go/src/fdb"

type RangeOptions struct {
	limit   *int
	reverse *bool
	mode    *fdb.StreamingMode
}

func (r *RangeOptions) SetLimit(limit int) {
	r.limit = &limit
}

func (r *RangeOptions) SetMode(mode fdb.StreamingMode) {
	r.mode = &mode
}

func (r *RangeOptions) SetReverse() {
	reverse := true
	r.reverse = &reverse
}

func SplitRangeOptions(opts []*RangeOptions) fdb.RangeOptions {
	res := fdb.RangeOptions{}

	for _, opt := range opts {
		if opt.limit != nil {
			res.Limit = *opt.limit
		}

		if opt.mode != nil {
			res.Mode = *opt.mode
		}

		if opt.reverse != nil {
			res.Reverse = *opt.reverse
		}
	}

	return res
}
