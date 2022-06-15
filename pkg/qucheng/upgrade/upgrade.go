// Copyright (c) 2021-2022 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
// Use of this source code is covered by the following dual licenses:
// (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
// (2) Affero General Public License 3.0 (AGPL 3.0)
// license that can be found in the LICENSE file.

package upgrade

import (
	"github.com/easysoft/qcadmin/common"
	"github.com/easysoft/qcadmin/internal/pkg/util/helm"
	"github.com/easysoft/qcadmin/internal/pkg/util/log"
)

func Upgrade(flagVersion string) error {
	helmClient, _ := helm.NewClient(&helm.Config{Namespace: common.DefaultSystem})
	if err := helmClient.UpdateRepo(); err != nil {
		log.Flog.Errorf("update repo failed, reason: %v", err)
		return err
	}

	if flagVersion == "" {
		// fetch
	}
	return nil
}

func QuchengVersion() (Version, error) {
	v := Version{}
	// TODO 获取渠成版本信息
	v.Components = append(v.Components, ComponentVersion{
		Name:         "qucheng_web",
		Version:      "0.1.5",
		AppVersion:   "1.0.0",
		ChartVersion: "0.1.5",
	}, ComponentVersion{
		Name:         "qucheng_api",
		Version:      "0.2.7",
		AppVersion:   "1.0.0",
		ChartVersion: "0.2.7",
	})
	return v, nil
}
