package mrcp

import (
	"reflect"
	"testing"
)

func Test_parseChannelId(t *testing.T) {
	type args struct {
		raw string
	}
	tests := []struct {
		name string
		args args
		want ChannelId
	}{
		{
			name: "parse",
			args: args{raw: "031691b2dcc7426f@speechsynth"},
			want: ChannelId{
				Id:       "031691b2dcc7426f",
				Resource: "speechsynth",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseChannelId(tt.args.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseChannelId() = %v, want %v", got, tt.want)
			}
		})
	}
}
