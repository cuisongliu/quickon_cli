// Copyright (c) 2021-2023 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
// Use of this source code is covered by the following dual licenses:
// (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
// (2) Affero General Public License 3.0 (AGPL 3.0)
// license that can be found in the LICENSE file.

package quickon

import (
	"github.com/easysoft/qcadmin/internal/pkg/types"
	"github.com/easysoft/qcadmin/pkg/providers"
)

const providerName = "quickon"

type Quickon struct {
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *Quickon {
	return &Quickon{}
}

func (q *Quickon) GetProviderName() string {
	return providerName
}

func (q *Quickon) GetFlags() []types.Flag {
	return nil
}

func (q *Quickon) Install() error {
	return nil
}

func (q *Quickon) Show() error {
	return nil
}
