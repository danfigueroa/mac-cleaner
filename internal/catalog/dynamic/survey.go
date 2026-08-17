package dynamic

import (
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// survey percorre o home uma única vez e responde às duas regras dinâmicas.
//
// Existe porque a alternativa custa caro de verdade. Com uma travessia por
// regra, medir este catálogo passou de 3 para 94 segundos numa máquina real — e
// não por dobrar o trabalho, mas porque duas varreduras profundas competindo com
// as 35 regras estáticas transformam leituras sequenciais em disputa por I/O. A
// soma isolada das partes era 29 segundos; juntas, davam 94.
//
// Uma passada só, e o resultado memorizado, resolve os dois problemas.
type survey struct {
	minimum    domain.Bytes
	staleAfter time.Duration

	once     sync.Once
	big      []domain.Target
	projects []domain.Target
	err      error
}

// run executa a varredura na primeira chamada e reaproveita o resultado depois.
func (s *survey) run(ctx context.Context, env domain.Env) error {
	s.once.Do(func() {
		s.err = s.walk(ctx, env)
	})
	return s.err
}

func (s *survey) walk(ctx context.Context, env domain.Env) error {
	limite := time.Now().Add(-s.staleAfter)

	err := walkHome(env.Home, func(path string, entry fs.DirEntry) bool {
		if ctx.Err() != nil {
			return true
		}

		if entry.IsDir() {
			return s.visitDir(path, entry, limite)
		}
		s.visitFile(path, entry)
		return false
	})
	if err != nil {
		return err
	}

	sort.SliceStable(s.big, func(i, j int) bool { return s.big[i].Size > s.big[j].Size })
	if len(s.big) > maxBigFiles {
		s.big = s.big[:maxBigFiles]
	}
	return nil
}

// visitDir trata os diretórios de build. Devolve true para não descer neles.
func (s *survey) visitDir(path string, entry fs.DirEntry, limite time.Time) bool {
	if _, isBuild := buildDirNames[entry.Name()]; !isBuild {
		return false
	}

	// Encontrado. Abaixo daqui são milhares de arquivos que não interessam
	// individualmente, e cujo tamanho o Scanner mede num passo só.
	projeto := filepath.Dir(path)
	if ultimoToque(projeto).After(limite) {
		return true
	}

	s.projects = append(s.projects, domain.Target{
		Path:  path,
		Label: filepath.Base(projeto) + "/" + entry.Name(),
	})
	return true
}

func (s *survey) visitFile(path string, entry fs.DirEntry) {
	info, err := entry.Info()
	if err != nil {
		return
	}
	// Symlinks não contam: o tamanho que importa é o do destino, e ele já
	// aparece no seu próprio caminho.
	if info.Mode()&fs.ModeSymlink != 0 {
		return
	}

	size := diskSize(info)
	if size < s.minimum {
		return
	}

	s.big = append(s.big, domain.Target{
		Path:  path,
		Label: filepath.Base(path),
		Size:  size,
		// Já sabemos o tamanho; medi-lo de novo seria um segundo stat por
		// arquivo, sem nenhum ganho.
		Measured: true,
	})
}

// Rules devolve as regras dinâmicas, já compartilhando uma varredura.
//
// As duas saem juntas de propósito: separá-las convidaria a construir uma sem a
// outra, e o custo da travessia voltaria a ser pago duas vezes.
func Rules(minimum domain.Bytes, staleAfter time.Duration) []domain.Rule {
	shared := &survey{minimum: minimum, staleAfter: staleAfter}

	return []domain.Rule{
		abandonedProjectsRule(shared, staleAfter),
		bigFilesRule(shared, minimum),
	}
}

// projectTargets e bigTargets são os Targets das duas regras. Ambos disparam a
// mesma varredura; a segunda chamada só lê o resultado já calculado.
func (s *survey) projectTargets(ctx context.Context, env domain.Env) ([]domain.Target, error) {
	if err := s.run(ctx, env); err != nil {
		return nil, err
	}
	return s.projects, nil
}

func (s *survey) bigTargets(ctx context.Context, env domain.Env) ([]domain.Target, error) {
	if err := s.run(ctx, env); err != nil {
		return nil, err
	}
	return s.big, nil
}
