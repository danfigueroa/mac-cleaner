//go:build darwin

package osfs

import (
	"time"

	"golang.org/x/sys/unix"
)

// timespecToTime converte um unix.Timespec.
//
// Existe como função nomeada porque os campos do Timespec mudam de tipo entre
// arquiteturas (int32 em 32 bits, int64 em 64), e concentrar a conversão aqui
// evita espalhar casts pelo adapter.
func timespecToTime(ts unix.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
