package domain

import "errors"

// Sentinelas do domínio. Todo erro que atravessa camadas envolve uma destas com
// %w, para que a CLI possa decidir o comportamento com errors.Is em vez de
// comparar strings.
var (
	// ErrGuardViolation indica que um caminho foi barrado pelo guard antes de
	// qualquer remoção. É o erro mais importante do programa: se ele aparecer,
	// há bug no catálogo, e o processo deve abortar em vez de seguir adiante.
	ErrGuardViolation = errors.New("caminho barrado pelo guard")

	// ErrNoFullDiskAccess sinaliza que o terminal não tem Acesso Total ao Disco
	// e parte de ~/Library ficou ilegível. Sem isso o relatório subestima o
	// espaço recuperável em silêncio, que é pior do que falhar.
	ErrNoFullDiskAccess = errors.New("sem Acesso Total ao Disco")

	// ErrRuleNotFound é devolvido quando `mac-cleaner clean <id>` recebe um ID
	// que não existe no catálogo.
	ErrRuleNotFound = errors.New("regra não encontrada")

	// ErrNeedsRoot indica tentativa de executar uma regra ManualOnly. A CLI
	// nunca escala privilégio.
	ErrNeedsRoot = errors.New("alvo exige root e não será removido automaticamente")

	// ErrInvalidSize vem de ParseBytes.
	ErrInvalidSize = errors.New("tamanho inválido")

	// ErrInvalidCategory vem da validação de --category.
	ErrInvalidCategory = errors.New("categoria inválida")

	// ErrNothingToClean encerra a execução com código 3: o disco já está limpo
	// segundo o catálogo, o que não é falha.
	ErrNothingToClean = errors.New("nada a limpar")
)
