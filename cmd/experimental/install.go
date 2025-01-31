// AGPL License
// Copyright (c) 2021 ysicing <i@ysicing.me>

package experimental

import (
	"fmt"
	"os"
	"runtime"

	"github.com/ergoapi/util/color"

	"github.com/cockroachdb/errors"
	"github.com/easysoft/qcadmin/common"
	"github.com/easysoft/qcadmin/internal/pkg/util/downloader"
	"github.com/easysoft/qcadmin/internal/pkg/util/factory"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	installExample = templates.Examples(`
		# install tools
		q experimental install helm`)
)

// InstallCommand install some tools
func InstallCommand(f factory.Factory) *cobra.Command {
	installCmd := &cobra.Command{
		Use:     "install [flags]",
		Short:   "install tools, like: helm, kubectl",
		Example: installExample,
		Args:    cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				f.GetLog().Fatalf("args error: %v", args)
				return errors.New("missing args: helm or kubectl")
			}
			tool := args[0]
			if tool != "helm" && tool != "kubectl" {
				return fmt.Errorf("not support tool: %s, only suppor helm, kubectl", tool)
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			f.GetLog().Info(args)
			tool := args[0]
			remoteURL := fmt.Sprintf("https://pkg.qucheng.com/qucheng/cli/stable/%s/%s-%s-%s", tool, tool, runtime.GOOS, runtime.GOARCH)
			localURL := fmt.Sprintf("%s/qc-%s", common.GetDefaultBinDir(), tool)
			res, err := downloader.Download(remoteURL, localURL)
			if err != nil {
				f.GetLog().Fatalf("download %s error: %v", tool, err)
				return
			}
			f.GetLog().Debugf("download %s result: %v", tool, res.Status)
			_ = os.Chmod(localURL, common.FileMode0755)
			f.GetLog().Donef(fmt.Sprintf("download %s success\n\tusage: %s", tool, color.SGreen("%s %s", os.Args[0], tool)))
		},
	}
	return installCmd
}
