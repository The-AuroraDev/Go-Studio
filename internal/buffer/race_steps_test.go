// race_steps_test.go — 竞态检测下的属性测试步数。
// SPDX-License-Identifier: MIT

//go:build race

package buffer

// propertySteps 竞态检测会显著拖慢代码，同样的迭代次数要跑上几分钟，
// 而门禁只需要覆盖到形状。所以这里削减步数，把完整强度留给 make test-deep。
const propertySteps = 400
