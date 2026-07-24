package utils

import (
	"fmt"
)

// SettingsDiffBuilder 配置差异日志构建器。
type SettingsDiffBuilder struct {
	diffs []string
}

// NewSettingsDiffBuilder 创建配置差异构建器实例。
func NewSettingsDiffBuilder() *SettingsDiffBuilder {
	return &SettingsDiffBuilder{
		diffs: make([]string, 0),
	}
}

// AddBool 校验并追加布尔型配置变更。
func (b *SettingsDiffBuilder) AddBool(label string, oldVal, newVal bool) {
	if oldVal != newVal {
		b.diffs = append(b.diffs, fmt.Sprintf("%s: %s -> %s", label, formatBool(oldVal), formatBool(newVal)))
	}
}

// AddInt 校验并追加整型配置变更。
func (b *SettingsDiffBuilder) AddInt(label string, oldVal, newVal int, unit string) {
	if oldVal != newVal {
		b.diffs = append(b.diffs, fmt.Sprintf("%s: %s -> %s", label, formatIntVal(oldVal, unit), formatIntVal(newVal, unit)))
	}
}

func formatIntVal(val int, unit string) string {
	if val < 0 {
		return "永久保留"
	}
	return fmt.Sprintf("%d%s", val, unit)
}

// AddString 校验并追加字符串型配置变更。
func (b *SettingsDiffBuilder) AddString(label string, oldVal, newVal string) {
	if oldVal != newVal {
		b.diffs = append(b.diffs, fmt.Sprintf("%s: %s -> %s", label, oldVal, newVal))
	}
}

// AddSecret 校验并追加脱敏凭据配置变更。
func (b *SettingsDiffBuilder) AddSecret(label string, oldVal, newVal string) {
	if newVal != "******" && oldVal != newVal {
		b.diffs = append(b.diffs, fmt.Sprintf("%s: [已更新]", label))
	}
}

// AddCustom 条件满足时追加自定义变更记录。
func (b *SettingsDiffBuilder) AddCustom(changed bool, detail string) {
	if changed {
		b.diffs = append(b.diffs, detail)
	}
}

// Log 统一输出配置变更日志。
func (b *SettingsDiffBuilder) Log(actionName string) {
	if len(b.diffs) == 0 {
		LogSuccess("%s (无配置变更)", actionName)
		return
	}
	LogSuccess("%s (修改了 %d 项配置)", actionName, len(b.diffs))
	for _, d := range b.diffs {
		LogInfo("修改配置项 [%s]", d)
	}
}

// FormatPeriod 格式化周期类型描述。
func FormatPeriod(cType string, cVal int) string {
	unitMap := map[string]string{
		"hour":  "小时",
		"day":   "天",
		"week":  "周",
		"month": "月",
	}
	unit, ok := unitMap[cType]
	if !ok {
		unit = cType
	}
	return fmt.Sprintf("每%d%s", cVal, unit)
}

func formatBool(v bool) string {
	if v {
		return "开启"
	}
	return "禁用"
}
