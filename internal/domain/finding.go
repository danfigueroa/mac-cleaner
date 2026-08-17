package domain

import (
	"sort"
	"time"
)

// Finding é o resultado de medir uma regra. É a unidade serializável — o cache
// de scan em disco guarda exatamente isto, e por isso ela referencia a regra por
// ID em vez de embutir a Rule (que carrega funcs e não sobrevive a um JSON).
type Finding struct {
	RuleID  string   `json:"rule_id"`
	Size    Bytes    `json:"size"`
	Targets []Target `json:"targets"`

	// Denied conta caminhos que existiam mas não puderam ser lidos, quase
	// sempre por falta de Acesso Total ao Disco. Size está subestimado quando
	// isto é maior que zero, e a interface precisa dizer isso.
	Denied int `json:"denied"`
}

// Empty informa se não há nada a recuperar nesta regra.
func (f Finding) Empty() bool { return f.Size == 0 && len(f.Targets) == 0 }

// Paths extrai os caminhos concretos, na ordem em que foram descobertos.
func (f Finding) Paths() []string {
	paths := make([]string, 0, len(f.Targets))
	for _, t := range f.Targets {
		paths = append(paths, t.Path)
	}
	return paths
}

// Result associa um Finding à sua Rule. É a junção usada por tudo que exibe algo
// — relatório, TUI, plano de limpeza — e existe só em memória.
type Result struct {
	Rule    Rule
	Finding Finding
}

// Volume descreve a ocupação do disco onde o home mora.
type Volume struct {
	Path  string `json:"path"`
	Total Bytes  `json:"total"`
	Free  Bytes  `json:"free"`
	Used  Bytes  `json:"used"`
}

// UsedPercent devolve a ocupação em porcentagem, para o cabeçalho da TUI.
func (v Volume) UsedPercent() float64 {
	if v.Total == 0 {
		return 0
	}
	return float64(v.Used) / float64(v.Total) * 100
}

// Report é a auditoria completa de uma varredura.
type Report struct {
	GeneratedAt time.Time
	Volume      Volume
	Results     []Result

	// DeniedPaths lista caminhos ilegíveis por falta de permissão. Guardamos os
	// caminhos, e não só a contagem, para que a mensagem final possa mostrar
	// exemplos concretos — "conceda Acesso Total ao Disco" é vago sozinho.
	DeniedPaths []string
}

// Reclaimable soma tudo que o relatório encontrou.
func (r Report) Reclaimable() Bytes {
	var total Bytes
	for _, res := range r.Results {
		total += res.Finding.Size
	}
	return total
}

// ReclaimableAtRisk soma apenas os alvos de um dado nível de risco — usado para
// mostrar quanto sai só com o que vem pré-marcado.
func (r Report) ReclaimableAtRisk(risk Risk) Bytes {
	var total Bytes
	for _, res := range r.Results {
		if res.Rule.Risk == risk {
			total += res.Finding.Size
		}
	}
	return total
}

// FilterBySize descarta os resultados abaixo do mínimo.
//
// Alvos de poucos megabytes não pagam a linha que ocupam na tela: eles diluem os
// que importam e fazem a lista parecer longa demais para ser lida. O total
// passa a refletir apenas o que ficou, que é o que o usuário pode de fato agir.
func (r *Report) FilterBySize(minimum Bytes) {
	if minimum <= 0 {
		return
	}

	kept := r.Results[:0]
	for _, result := range r.Results {
		if result.Finding.Size >= minimum {
			kept = append(kept, result)
		}
	}
	r.Results = kept
}

// SortBySize ordena os resultados do maior para o menor, que é a única ordem
// útil numa ferramenta de liberar espaço.
func (r *Report) SortBySize() {
	sort.SliceStable(r.Results, func(i, j int) bool {
		return r.Results[i].Finding.Size > r.Results[j].Finding.Size
	})
}

// GroupByCategory agrupa os resultados na ordem canônica das categorias,
// omitindo as vazias. Dentro de cada grupo, do maior para o menor.
func (r Report) GroupByCategory() []CategoryGroup {
	byCategory := make(map[Category][]Result, len(Categories()))
	for _, res := range r.Results {
		byCategory[res.Rule.Category] = append(byCategory[res.Rule.Category], res)
	}

	groups := make([]CategoryGroup, 0, len(Categories()))
	for _, category := range Categories() {
		results := byCategory[category]
		if len(results) == 0 {
			continue
		}
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].Finding.Size > results[j].Finding.Size
		})

		var total Bytes
		for _, res := range results {
			total += res.Finding.Size
		}
		groups = append(groups, CategoryGroup{Category: category, Results: results, Total: total})
	}
	return groups
}

// CategoryGroup é uma seção do relatório.
type CategoryGroup struct {
	Category Category
	Results  []Result
	Total    Bytes
}
