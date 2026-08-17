package catalog

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// dockerRules cobre o Docker Desktop.
//
// Cada tipo de resíduo é uma regra separada em vez de um `docker system prune`
// único, porque eles não têm o mesmo risco: cache de build é descartável,
// enquanto volumes guardam bancos de dados inteiros. Juntá-los faria a única
// decisão disponível ser a mais perigosa das quatro.
func dockerRules() []domain.Rule {
	return []domain.Rule{
		{
			ID:       "docker-buildcache",
			Name:     "Docker — cache de build",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What:     "Camadas intermediárias que o BuildKit guarda para acelerar rebuilds.",
			Lose:     "Nada. É resultado derivado dos seus Dockerfiles.",
			Regen:    "O próximo build reconstrói as camadas, uma vez só.",
			Detect:   hasTool("docker"),
			Targets:  dockerReclaimable("Build Cache"),
			Strategy: domain.RunCommand{Name: "docker", Args: []string{"builder", "prune", "-a", "-f"}},
		},
		{
			ID:       "docker-containers",
			Name:     "Docker — contêineres parados",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Contêineres que terminaram e nunca foram removidos.",
			Lose: "Os arquivos escritos dentro do contêiner fora de volumes, e os logs dele. " +
				"Contêineres em execução não são tocados.",
			Regen:    "`docker run` ou `docker compose up` cria novos a partir das imagens.",
			Detect:   hasTool("docker"),
			Targets:  dockerReclaimable("Containers"),
			Strategy: domain.RunCommand{Name: "docker", Args: []string{"container", "prune", "-f"}},
		},
		{
			ID:       "docker-images",
			Name:     "Docker — imagens sem uso",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Imagens que nenhum contêiner referencia, incluindo camadas soltas de builds antigos.",
			Lose:     "Imagens construídas localmente e nunca enviadas para um registro precisam ser refeitas.",
			Regen:    "`docker pull` rebaixa as públicas; as suas voltam com um novo build.",
			Detect:   hasTool("docker"),
			Targets:  dockerReclaimable("Images"),
			Strategy: domain.RunCommand{Name: "docker", Args: []string{"image", "prune", "-a", "-f"}},
		},
		{
			ID:       "docker-volumes",
			Name:     "Docker — volumes órfãos",
			Category: domain.CategoryDev,
			Risk:     domain.RiskReview,
			What:     "Volumes que nenhum contêiner usa no momento.",
			Lose: "Dados de verdade. Um banco de dados de ambiente local mora num volume, " +
				"e ele aparece como órfão sempre que o contêiner correspondente está parado.",
			Regen:    "Não volta. O conteúdo é apagado de forma definitiva.",
			Detect:   hasTool("docker"),
			Targets:  dockerReclaimable("Local Volumes"),
			Strategy: domain.RunCommand{Name: "docker", Args: []string{"volume", "prune", "-f"}},
		},
	}
}

// dockerDF é uma linha da saída de `docker system df --format "{{json .}}"`.
type dockerDF struct {
	Type        string `json:"Type"`
	Size        string `json:"Size"`
	Reclaimable string `json:"Reclaimable"`
}

// dockerReclaimable pergunta ao daemon quanto ele consegue liberar de um tipo.
//
// O número vem do próprio Docker, e não de medir diretórios: as camadas ficam
// dentro da imagem de disco da VM do Docker Desktop, onde o tamanho do arquivo
// não diz nada sobre quanto está em uso lá dentro.
func dockerReclaimable(wantType string) targetsFunc {
	return func(ctx context.Context, env domain.Env) ([]domain.Target, error) {
		if env.Query == nil {
			return nil, nil
		}

		out, err := env.Query(ctx, "docker", "system", "df", "--format", "{{json .}}")
		if err != nil {
			// O caso comum é o daemon estar parado — o binário existe, mas não
			// há a quem perguntar. Não é erro: a regra apenas não se aplica
			// agora, e propagar isso derrubaria a varredura das outras 34 regras
			// só porque o Docker Desktop não estava aberto.
			//nolint:nilerr // daemon indisponível não é falha da varredura
			return nil, nil
		}

		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			var line dockerDF
			if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
				continue
			}
			if line.Type != wantType {
				continue
			}

			size := parseDockerSize(line.Reclaimable)
			if size == 0 {
				return nil, nil
			}
			return []domain.Target{{
				// Sem Path: quem sabe o que remover é o próprio daemon, e a
				// remoção acontece pelo comando oficial. Inventar um caminho aqui
				// só criaria a ilusão de que há um diretório a apagar.
				Label:    wantType,
				Size:     size,
				Measured: true,
			}}, nil
		}
		return nil, nil
	}
}

// parseDockerSize interpreta o campo Reclaimable, que vem como "1.2GB" ou
// "800MB (66%)".
func parseDockerSize(raw string) domain.Bytes {
	value, _, _ := strings.Cut(strings.TrimSpace(raw), " ")

	size, err := domain.ParseBytes(value)
	if err != nil {
		return 0
	}
	return size
}
