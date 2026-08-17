package domain_test

import (
	"errors"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

func TestBytesString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   domain.Bytes
		want string
	}{
		{"zero", 0, "0 B"},
		{"abaixo de 1 KB", 512, "512 B"},
		{"limite do KB", 1_000, "1.0 KB"},
		{"megabytes", 547 * domain.Megabyte, "547 MB"},
		{"gigabytes com decimal", 7_300_000_000, "7.3 GB"},
		{"acima de 100 perde a decimal", 184 * domain.Gigabyte, "184 GB"},
		{"terabytes", 2 * domain.Terabyte, "2.0 TB"},
		{"negativo", -1500, "-1.5 KB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.String(); got != tt.want {
				t.Errorf("Bytes(%d).String() = %q, quer %q", int64(tt.in), got, tt.want)
			}
		})
	}
}

func TestParseBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want domain.Bytes
	}{
		{"sem unidade é byte", "1024", 1024},
		{"decimal maiúsculo", "500MB", 500 * domain.Megabyte},
		{"decimal minúsculo", "500mb", 500 * domain.Megabyte},
		{"sufixo curto", "2G", 2 * domain.Gigabyte},
		{"fracionado", "1.5G", 1_500_000_000},
		{"binário é aceito na entrada", "1GiB", 1 << 30},
		{"espaço em volta", "  10 MB  ", 10 * domain.Megabyte},
		{"apenas B", "42B", 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseBytes(tt.in)
			if err != nil {
				t.Fatalf("ParseBytes(%q) devolveu erro inesperado: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseBytes(%q) = %d, quer %d", tt.in, int64(got), int64(tt.want))
			}
		})
	}
}

func TestParseBytesRejeita(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "  ", "abc", "-5MB", "MB", "1,5G"} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.ParseBytes(in); !errors.Is(err, domain.ErrInvalidSize) {
				t.Errorf("ParseBytes(%q) devolveu %v, quer ErrInvalidSize", in, err)
			}
		})
	}
}

// TestParseBytesIdaEVolta garante que o que formatamos continua legível para nós
// mesmos — a TUI exibe um tamanho e o usuário pode colá-lo de volta numa flag.
func TestParseBytesIdaEVolta(t *testing.T) {
	t.Parallel()

	for _, size := range []domain.Bytes{512, 1_000, 15 * domain.Megabyte, 2 * domain.Gigabyte} {
		t.Run(size.String(), func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseBytes(size.String())
			if err != nil {
				t.Fatalf("ParseBytes(%q): %v", size.String(), err)
			}
			if got != size {
				t.Errorf("ida e volta de %d virou %d", int64(size), int64(got))
			}
		})
	}
}
