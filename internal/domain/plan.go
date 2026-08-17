package domain

// Plan é o conjunto de ações que o usuário aprovou. Ele é construído,
// inteiramente validado e só então executado — nunca se decide o que remover no
// meio da execução.
type Plan struct {
	Items []PlanItem
}

// PlanItem é uma regra selecionada junto com o que foi medido nela.
type PlanItem struct {
	Rule    Rule
	Finding Finding
}

// Total soma o espaço que o plano promete recuperar.
func (p Plan) Total() Bytes {
	var total Bytes
	for _, item := range p.Items {
		total += item.Finding.Size
	}
	return total
}

// Empty informa se não há nada a fazer.
func (p Plan) Empty() bool { return len(p.Items) == 0 }

// TrashTotal soma o que irá para a Lixeira, em vez de ser removido por comando
// nativo.
//
// É o número que a interface precisa destacar no fim: esse espaço NÃO é
// liberado até a Lixeira ser esvaziada. Sem esse aviso o usuário roda `df`, não
// vê diferença e conclui que a ferramenta não funciona.
func (p Plan) TrashTotal() Bytes {
	var total Bytes
	for _, item := range p.Items {
		if _, ok := item.Rule.Strategy.(TrashTargets); ok {
			total += item.Finding.Size
		}
	}
	return total
}

// Outcome registra o que de fato aconteceu com um item do plano.
type Outcome struct {
	Rule Rule

	// Reclaimed é o tamanho que o item representava. "Recuperado" aqui significa
	// removido do lugar original — se foi para a Lixeira, o espaço em disco só
	// volta ao esvaziá-la.
	Reclaimed Bytes

	// Trashed distingue os dois destinos, para que o resumo final não misture
	// espaço já livre com espaço apenas movido.
	Trashed bool

	Err error
}

// Summary é o resultado da execução de um Plan.
type Summary struct {
	Outcomes []Outcome
}

// Reclaimed soma o que foi de fato liberado no disco agora.
func (s Summary) Reclaimed() Bytes {
	var total Bytes
	for _, o := range s.Outcomes {
		if o.Err == nil && !o.Trashed {
			total += o.Reclaimed
		}
	}
	return total
}

// Trashed soma o que foi movido para a Lixeira e ainda ocupa disco.
func (s Summary) Trashed() Bytes {
	var total Bytes
	for _, o := range s.Outcomes {
		if o.Err == nil && o.Trashed {
			total += o.Reclaimed
		}
	}
	return total
}

// Failures devolve os itens que falharam.
func (s Summary) Failures() []Outcome {
	var failures []Outcome
	for _, o := range s.Outcomes {
		if o.Err != nil {
			failures = append(failures, o)
		}
	}
	return failures
}
