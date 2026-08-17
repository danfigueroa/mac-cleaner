package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Bytes é um tamanho em bytes.
//
// A formatação usa base decimal (1 GB = 1.000.000.000 bytes), igual ao Finder e
// ao painel Armazenamento das Ajustes do Sistema. Isso é deliberado: o usuário
// compara o resultado desta CLI com o que o macOS mostra, não com `du -h` (que é
// base 1024). Em escala de GB a diferença passa de 7% — o bastante para o
// relatório parecer errado.
type Bytes int64

// Múltiplos decimais.
const (
	Kilobyte Bytes = 1_000
	Megabyte Bytes = 1_000 * Kilobyte
	Gigabyte Bytes = 1_000 * Megabyte
	Terabyte Bytes = 1_000 * Gigabyte
)

// String formata o tamanho de forma legível: "512 B", "7.3 GB", "184 GB".
//
// A precisão cai para zero casas acima de 100 unidades, porque "184.2 GB" só
// adiciona ruído numa coluna que o usuário lê de relance.
func (b Bytes) String() string {
	if b < 0 {
		return "-" + (-b).String()
	}
	if b < Kilobyte {
		return strconv.FormatInt(int64(b), 10) + " B"
	}

	unit, symbol := Kilobyte, "KB"
	switch {
	case b >= Terabyte:
		unit, symbol = Terabyte, "TB"
	case b >= Gigabyte:
		unit, symbol = Gigabyte, "GB"
	case b >= Megabyte:
		unit, symbol = Megabyte, "MB"
	}

	value := float64(b) / float64(unit)
	precision := 1
	if value >= 100 {
		precision = 0
	}
	return strconv.FormatFloat(value, 'f', precision, 64) + " " + symbol
}

// unitSuffixes mapeia sufixos aceitos por ParseBytes para seu multiplicador.
//
// Aceitamos tanto os decimais (KB, MB) quanto os binários (KiB, MiB) porque o
// usuário digita o que tem na cabeça. A saída, porém, é sempre decimal.
var unitSuffixes = []struct {
	suffix string
	mult   Bytes
}{
	{"KIB", 1 << 10},
	{"MIB", 1 << 20},
	{"GIB", 1 << 30},
	{"TIB", 1 << 40},
	{"KB", Kilobyte},
	{"MB", Megabyte},
	{"GB", Gigabyte},
	{"TB", Terabyte},
	{"K", Kilobyte},
	{"M", Megabyte},
	{"G", Gigabyte},
	{"T", Terabyte},
	{"B", 1},
}

// ParseBytes interpreta tamanhos escritos por humanos: "500MB", "1.5G", "2GiB", "1024".
func ParseBytes(s string) (Bytes, error) {
	normalized := strings.ToUpper(strings.TrimSpace(s))
	if normalized == "" {
		return 0, fmt.Errorf("%w: tamanho vazio", ErrInvalidSize)
	}

	mult := Bytes(1)
	for _, u := range unitSuffixes {
		if rest, ok := strings.CutSuffix(normalized, u.suffix); ok {
			normalized, mult = strings.TrimSpace(rest), u.mult
			break
		}
	}

	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidSize, s)
	}
	if value < 0 {
		return 0, fmt.Errorf("%w: %q é negativo", ErrInvalidSize, s)
	}
	return Bytes(value * float64(mult)), nil
}

// Set implementa pflag.Value, permitindo usar Bytes direto como flag.
func (b *Bytes) Set(s string) error {
	parsed, err := ParseBytes(s)
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}

// Type implementa pflag.Value.
func (b *Bytes) Type() string { return "size" }
