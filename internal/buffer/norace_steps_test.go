// norace_steps_test.go — 非竞态检测下的属性测试步数。
// SPDX-License-Identifier: MIT

//go:build !race

package buffer

// propertySteps 是普通运行时每个种子的随机编辑次数。
const propertySteps = 2000
