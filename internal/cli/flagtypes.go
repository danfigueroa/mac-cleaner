package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// durationValue é um pflag.Value para durações que também aceita dias.
//
// time.ParseDuration não conhece "d", mas "90d" é exatamente como alguém pensa
// em "projeto abandonado". Escrever isso como "2160h" seria fiel ao Go e hostil
// ao usuário.
type durationValue struct{ target *time.Duration }

func newDurationValue(target *time.Duration) *durationValue {
	return &durationValue{target: target}
}

func (d *durationValue) String() string {
	if d.target == nil || *d.target == 0 {
		return "0"
	}
	// Devolve em dias quando a duração é um número exato deles, para que o
	// --help mostre "90d" e não "2160h0m0s".
	const day = 24 * time.Hour
	if *d.target%day == 0 {
		return strconv.FormatInt(int64(*d.target/day), 10) + "d"
	}
	return d.target.String()
}

func (d *durationValue) Set(s string) error {
	s = strings.TrimSpace(s)

	if rest, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return fmt.Errorf("%w: duração %q inválida", errUsage, s)
		}
		if days < 0 {
			return fmt.Errorf("%w: duração %q é negativa", errUsage, s)
		}
		*d.target = time.Duration(days * float64(24*time.Hour))
		return nil
	}

	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("%w: duração %q inválida (use 90d, 12h, 30m)", errUsage, s)
	}
	if parsed < 0 {
		return fmt.Errorf("%w: duração %q é negativa", errUsage, s)
	}
	*d.target = parsed
	return nil
}

// Type é exibido no --help. Fica em ASCII porque o pflag alinha as colunas
// contando bytes, não runes: um acento aqui desalinha a tabela inteira.
func (d *durationValue) Type() string { return "duration" }
