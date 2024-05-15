package migrations

import (
	"reflect"
	"testing"
)

func TestMigrations(t *testing.T) {
	type args struct {
		version uint32
	}

	tests := []struct {
		name string
		args args
		want []Migration
	}{
		{
			name: "0",
			args: args{
				version: 0,
			},
			want: []Migration{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Migrations(tt.args.version); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Migrations() = %v, want %v", got, tt.want)
			}
		})
	}
}
