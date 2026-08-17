// Package report renderiza uma auditoria em formatos legíveis por gente e por
// máquina.
package report

import (
	"encoding/json"
	"io"
	"time"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// jsonReport é o formato estável da saída `--json`.
//
// É um DTO separado de domain.Report de propósito: alguém vai versionar um
// relatório num repositório ou compará-lo entre execuções, e essa saída não pode
// mudar toda vez que um campo interno for renomeado.
type jsonReport struct {
	GeneratedAt      time.Time     `json:"generated_at"`
	Volume           jsonVolume    `json:"volume"`
	ReclaimableBytes int64         `json:"reclaimable_bytes"`
	Reclaimable      string        `json:"reclaimable"`
	Findings         []jsonFinding `json:"findings"`

	// DeniedPaths não é decorativo: quando vem preenchido, todos os tamanhos
	// acima estão subestimados, e quem consome o JSON precisa poder saber disso.
	DeniedPaths []string `json:"denied_paths,omitempty"`
}

type jsonVolume struct {
	Path        string  `json:"path"`
	TotalBytes  int64   `json:"total_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
	FreeBytes   int64   `json:"free_bytes"`
	Total       string  `json:"total"`
	Used        string  `json:"used"`
	Free        string  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

type jsonFinding struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Category  string   `json:"category"`
	Risk      string   `json:"risk"`
	SizeBytes int64    `json:"size_bytes"`
	Size      string   `json:"size"`
	What      string   `json:"what"`
	Lose      string   `json:"lose"`
	Regen     string   `json:"regen"`
	Command   string   `json:"command"`
	NeedsRoot bool     `json:"needs_root"`
	Paths     []string `json:"paths,omitempty"`
}

// JSON escreve o relatório como JSON indentado.
func JSON(w io.Writer, rep domain.Report) error {
	findings := make([]jsonFinding, 0, len(rep.Results))
	for _, result := range rep.Results {
		findings = append(findings, jsonFinding{
			ID:        result.Rule.ID,
			Name:      result.Rule.Name,
			Category:  string(result.Rule.Category),
			Risk:      result.Rule.Risk.String(),
			SizeBytes: int64(result.Finding.Size),
			Size:      result.Finding.Size.String(),
			What:      result.Rule.What,
			Lose:      result.Rule.Lose,
			Regen:     result.Rule.Regen,
			Command:   domain.CommandPreview(result),
			NeedsRoot: result.Rule.NeedsRoot(),
			Paths:     result.Finding.Paths(),
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	// Sem escapar HTML: caminhos contêm & e < com alguma frequência, e
	// transformá-los em & tornaria a saída pior de ler sem ganho algum.
	encoder.SetEscapeHTML(false)

	return encoder.Encode(jsonReport{
		GeneratedAt: rep.GeneratedAt,
		Volume: jsonVolume{
			Path:        rep.Volume.Path,
			TotalBytes:  int64(rep.Volume.Total),
			UsedBytes:   int64(rep.Volume.Used),
			FreeBytes:   int64(rep.Volume.Free),
			Total:       rep.Volume.Total.String(),
			Used:        rep.Volume.Used.String(),
			Free:        rep.Volume.Free.String(),
			UsedPercent: rep.Volume.UsedPercent(),
		},
		ReclaimableBytes: int64(rep.Reclaimable()),
		Reclaimable:      rep.Reclaimable().String(),
		Findings:         findings,
		DeniedPaths:      rep.DeniedPaths,
	})
}
