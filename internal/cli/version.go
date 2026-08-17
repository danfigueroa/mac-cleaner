package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version é sobrescrita no build de release via
// -ldflags "-X github.com/danfigueroa/mac-cleaner/internal/cli.version=v1.2.3".
// Fora disso, resolveVersion lê os metadados que o próprio `go build` embute.
var version string

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Mostra a versão do mac-cleaner",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "mac-cleaner %s (%s/%s, %s)\n",
				resolveVersion(), runtime.GOOS, runtime.GOARCH, runtime.Version())
			return err
		},
	}
}

// resolveVersion descobre a versão sem depender de um build script.
//
// Quem instala com `go install ...@latest` recebe a tag do módulo; quem compila
// o repo local recebe o hash do commit. Reportar "desconhecida" para um binário
// compilado do fonte tornaria qualquer relato de bug inútil.
func resolveVersion() string {
	if version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "desconhecida"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" {
		return "devel"
	}

	var sb strings.Builder
	sb.WriteString("devel-")
	sb.WriteString(revision[:min(len(revision), 12)])
	if modified == "true" {
		sb.WriteString("-sujo")
	}
	return sb.String()
}
