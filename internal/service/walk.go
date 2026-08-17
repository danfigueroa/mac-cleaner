package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// treeResult é o que uma subárvore contribuiu para a medição.
type treeResult struct {
	size domain.Bytes

	// denied são caminhos que existem mas não puderam ser lidos. Quase sempre
	// significam falta de Acesso Total ao Disco, e precisam chegar até a
	// interface: um total silenciosamente subestimado é pior que um erro.
	denied []string
}

func (r *treeResult) merge(other treeResult) {
	r.size += other.size
	r.denied = append(r.denied, other.denied...)
}

// treeState é o estado compartilhado de uma travessia.
type treeState struct {
	// device fixa o volume da raiz. Entradas em outro device são ignoradas:
	// no macOS o sistema aparece montado dentro da árvore por firmlinks, e
	// descer neles significaria medir o SO inteiro achando que é cache.
	device uint64

	mu   sync.Mutex
	seen map[uint64]struct{}

	// sem limita quantas subárvores são percorridas em paralelo. É o mesmo
	// canal para o scan inteiro, então o teto vale para todas as regras juntas.
	sem chan struct{}
}

// claim informa se este arquivo ainda não foi contado nesta travessia.
func (st *treeState) claim(info FileInfo) bool {
	// Diretórios sempre têm Links >= 2 no Unix ("." e a entrada no pai), e
	// hardlink de diretório não existe no APFS. Deduplicá-los só encheria o mapa.
	if info.IsDir || info.Links <= 1 {
		return true
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if _, counted := st.seen[info.Inode]; counted {
		return false
	}
	st.seen[info.Inode] = struct{}{}
	return true
}

// tryAcquire pega um slot de paralelismo se houver um livre. Nunca bloqueia:
// sem slot, a travessia continua na mesma goroutine. É isso que evita tanto a
// explosão de goroutines quanto o deadlock de uma recursão esperando por si mesma.
func (st *treeState) tryAcquire() bool {
	select {
	case st.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (st *treeState) release() { <-st.sem }

// walkDir soma o espaço ocupado pelo conteúdo de dir, recursivamente.
func (s *Scanner) walkDir(ctx context.Context, dir string, st *treeState) (treeResult, error) {
	if err := ctx.Err(); err != nil {
		return treeResult{}, err
	}

	names, err := s.fs.ReadDir(dir)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrPermission):
		// Registra e segue. Abortar o scan inteiro porque um diretório do
		// sistema é ilegível deixaria a ferramenta inútil em qualquer Mac sem
		// Acesso Total ao Disco — que é a configuração padrão.
		return treeResult{denied: []string{dir}}, nil
	case errors.Is(err, fs.ErrNotExist):
		// Corrida com outro processo apagando arquivos durante a varredura.
		return treeResult{}, nil
	default:
		return treeResult{}, fmt.Errorf("lendo diretório %s: %w", dir, err)
	}

	var (
		mu      sync.Mutex
		result  treeResult
		inlnErr error
	)

	group, groupCtx := errgroup.WithContext(ctx)

	for _, name := range names {
		child := filepath.Join(dir, name)

		info, statErr := s.fs.Lstat(child)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrPermission) {
				mu.Lock()
				result.denied = append(result.denied, child)
				mu.Unlock()
			}
			// NotExist e demais erros de entrada individual são ignorados: um
			// arquivo que sumiu no meio da varredura não invalida o resto.
			continue
		}

		if !st.claim(info) {
			continue
		}

		mu.Lock()
		result.size += info.Size
		mu.Unlock()

		// Symlinks têm IsDir falso no Lstat, então não são seguidos. Isso é
		// deliberado: seguir um link em ~/Library para /Applications faria o
		// scanner medir aplicativos e oferecê-los para remoção.
		if !info.IsDir || info.Device != st.device {
			continue
		}

		if st.tryAcquire() {
			group.Go(func() error {
				defer st.release()

				sub, err := s.walkDir(groupCtx, child, st)
				if err != nil {
					return err
				}
				mu.Lock()
				result.merge(sub)
				mu.Unlock()
				return nil
			})
			continue
		}

		sub, err := s.walkDir(groupCtx, child, st)
		if err != nil {
			inlnErr = err
			break
		}
		mu.Lock()
		result.merge(sub)
		mu.Unlock()
	}

	// group.Wait vem antes de qualquer retorno de erro: sair daqui com
	// goroutines ainda escrevendo em result seria corrida de dados.
	if err := group.Wait(); err != nil {
		return treeResult{}, err
	}
	if inlnErr != nil {
		return treeResult{}, inlnErr
	}
	return result, nil
}
