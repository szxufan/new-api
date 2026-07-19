package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMd5(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		want  string
	}{
		{
			name: "empty",
			data: []byte(""),
			want: "d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name: "abc",
			data: []byte("abc"),
			want: "900150983cd24fb0d6963f7d28e17f72",
		},
		{
			name: "hello world",
			data: []byte("hello world"),
			want: "5eb63bbbe01eeed093cb22bb8f5acdc3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Md5(tt.data)
			require.Equal(t, tt.want, got)
		})
	}
}
