// Copyright (c) 2021-2023 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
// Use of this source code is covered by the following dual licenses:
// (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
// (2) Affero General Public License 3.0 (AGPL 3.0)
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/easysoft/qcadmin/cmd/debug"
	"github.com/easysoft/qcadmin/internal/pkg/util/factory"
	"github.com/spf13/cobra"
)

func newCmdDebug(f factory.Factory) *cobra.Command {
	debugCmd := &cobra.Command{
		Use:    "debug",
		Hidden: true,
		Short:  "debug, not a stable interface, contains misc debug facilities",
		Long:   fmt.Sprintf("\"%s debug\" contains misc debug facilities; it is not a stable interface.", os.Args[0]),
	}
	debugCmd.AddCommand(debug.HostInfoCommand(f))
	debugCmd.AddCommand(debug.IngressNoHostCommand(f))
	debugCmd.AddCommand(debug.GOpsCommand(f))
	debugCmd.AddCommand(debug.NetcheckCommand(f))
	debugCmd.AddCommand(debug.PortForwardCommand(f))
	return debugCmd
}
