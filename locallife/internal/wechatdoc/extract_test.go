package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractMarkdown_BestEffortStructuredOutput(t *testing.T) {
	markdown := `# 微信支付

## 创建分账

请求方式：POST
请求URL：/v3/profitsharing/orders

### 请求参数

| 字段名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string | 是 | 子商户号 |
| receiver_type | string | 条件必填 | 当接收方是个人时必填。可选值：MERCHANT_ID/PERSONAL_OPENID |

### 返回参数

| 字段 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| order_id | string | 是 | 微信分账单号 |
| status | string | 是 | 分账单状态。可选值：PROCESSING/FINISHED |

### 状态说明

| 状态 | 说明 |
| --- | --- |
| PROCESSING | 处理中 |
| FINISHED | 已完成 |

### 错误码

| 错误码 | 含义 | 解决方案 |
| --- | --- | --- |
| PARAM_ERROR | 参数错误 | 请检查参数 |
| NO_AUTH | 无权限 | 请检查权限配置 |
`

	result := ExtractMarkdown(markdown)
	require.NotNil(t, result)
	require.Equal(t, 5, result.Summary.SectionCount)
	require.Equal(t, 1, result.Summary.EndpointCount)
	require.Equal(t, 4, result.Summary.FieldCount)
	require.Equal(t, 1, result.Summary.EnumSetCount)
	require.Equal(t, 2, result.Summary.EnumValueCount)
	require.Equal(t, 2, result.Summary.ErrorCodeCount)
	require.Empty(t, result.UnknownTables)

	requestSection := findSectionByHeading(result.Sections, "请求参数")
	require.NotNil(t, requestSection)
	require.Len(t, requestSection.Fields, 2)
	require.Equal(t, "required", requestSection.Fields[0].Requirement)
	require.Equal(t, "conditional", requestSection.Fields[1].Requirement)
	require.Contains(t, requestSection.Fields[1].Condition, "当接收方是个人时必填")
	require.Equal(t, []string{"MERCHANT_ID", "PERSONAL_OPENID"}, requestSection.Fields[1].EnumValues)

	responseSection := findSectionByHeading(result.Sections, "返回参数")
	require.NotNil(t, responseSection)
	require.Len(t, responseSection.Fields, 2)
	require.Equal(t, []string{"PROCESSING", "FINISHED"}, responseSection.Fields[1].EnumValues)

	statusSection := findSectionByHeading(result.Sections, "状态说明")
	require.NotNil(t, statusSection)
	require.Len(t, statusSection.EnumSets, 1)
	require.Equal(t, "status", statusSection.EnumSets[0].Kind)
	require.Len(t, statusSection.EnumSets[0].Values, 2)

	errorSection := findSectionByHeading(result.Sections, "错误码")
	require.NotNil(t, errorSection)
	require.Len(t, errorSection.ErrorCodes, 2)
	require.Equal(t, "PARAM_ERROR", errorSection.ErrorCodes[0].Code)

	rootSection := findSectionByHeading(result.Sections, "创建分账")
	require.NotNil(t, rootSection)
	require.Len(t, rootSection.Endpoints, 1)
	require.Equal(t, "POST", rootSection.Endpoints[0].Method)
	require.Equal(t, "/v3/profitsharing/orders", rootSection.Endpoints[0].Path)
}

func TestExtractMarkdown_WechatMethodAndStandalonePathPattern(t *testing.T) {
	markdown := "# 商户进件\n\n" +
		"## 提交申请单\n\n" +
		"## 接口说明\n\n" +
		"支持商户：【平台商户】\n\n" +
		"请求方式：【POST】\n" +
		"`/v3/ecommerce/applyments/`\n\n" +
		"## 查询结算账户\n\n" +
		"请求方式：【GET】\n" +
		"`/v3/apply4sub/sub_merchants/{sub_mchid}/settlement`\n"

	result := ExtractMarkdown(markdown)
	require.NotNil(t, result)

	submitSection := findSectionByPath(result.Sections, []string{"商户进件", "接口说明"})
	require.NotNil(t, submitSection)
	require.Len(t, submitSection.Endpoints, 1)
	require.Equal(t, "POST", submitSection.Endpoints[0].Method)
	require.Equal(t, "/v3/ecommerce/applyments/", submitSection.Endpoints[0].Path)

	querySection := findSectionByPath(result.Sections, []string{"商户进件", "查询结算账户"})
	require.NotNil(t, querySection)
	require.Len(t, querySection.Endpoints, 1)
	require.Equal(t, "GET", querySection.Endpoints[0].Method)
	require.Equal(t, "/v3/apply4sub/sub_merchants/{sub_mchid}/settlement", querySection.Endpoints[0].Path)
}

func TestExtractMarkdown_RequirementInferenceAvoidsFalsePositiveShi(t *testing.T) {
	markdown := `# 商户进件

## 提交申请单

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| finance_institution | boolean | false | 是否金融机构 |
| store_url | string | false | 店铺二维码或店铺链接二选一必填 |
`

	result := ExtractMarkdown(markdown)
	section := findSectionByHeading(result.Sections, "Body 参数")
	require.NotNil(t, section)
	require.Len(t, section.Fields, 2)
	require.Equal(t, "optional", section.Fields[0].Requirement)
	require.Equal(t, "conditional", section.Fields[1].Requirement)
	require.Contains(t, section.Fields[1].Condition, "二选一必填")
}

func findSectionByHeading(sections []Section, heading string) *Section {
	for index := range sections {
		if sections[index].Heading == heading {
			return &sections[index]
		}
	}
	return nil
}

func findSectionByPath(sections []Section, path []string) *Section {
	for index := range sections {
		if len(sections[index].Path) != len(path) {
			continue
		}
		matched := true
		for pathIndex := range path {
			if sections[index].Path[pathIndex] != path[pathIndex] {
				matched = false
				break
			}
		}
		if matched {
			return &sections[index]
		}
	}
	return nil
}
