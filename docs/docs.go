// Copyright (c) 2021-2023 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
// Use of this source code is covered by the following dual licenses:
// (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
// (2) Affero General Public License 3.0 (AGPL 3.0)
// license that can be found in the LICENSE file.

package main

import (
	"strings"

	"github.com/easysoft/qcadmin/cmd"
	"github.com/easysoft/qcadmin/internal/pkg/util/factory"
	"github.com/ergoapi/util/file"
	"github.com/ergoapi/util/github"
	"github.com/ergoapi/util/version"
	"github.com/spf13/cobra/doc"
)

func main() {
	f := factory.DefaultFactory()
	q := cmd.BuildRoot(f)
	err := doc.GenMarkdownTree(q, "./docs")
	if err != nil {
		panic(err)
	}
	pkg := github.Pkg{
		Owner: "easysoft",
		Repo:  "quickon_cli",
	}
	tag, err := pkg.LastTag()
	if err != nil {
		return
	}
	file.WriteFile("VERSION", strings.TrimPrefix(version.Next(tag.Name, false, false, true), "v"), true)
}
