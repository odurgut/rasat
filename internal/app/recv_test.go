package app

import "testing"

func TestRecvSize(t *testing.T) {
	t.Parallel()
	if got := recvSize(0); got != 16<<20 {
		t.Fatalf("default %d", got)
	}
	if got := recvSize(1024); got != 1024 {
		t.Fatalf("passthrough %d", got)
	}
	if got := recvSize(128 << 20); got != 64<<20 {
		t.Fatalf("cap %d", got)
	}
}
