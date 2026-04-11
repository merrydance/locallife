# OCR 样本集与基线评估

## 1. 目的

本文对应执行计划 T47 / T47A，用于固定 OCR 样本集结构、评估脚本入口和核心指标口径。

目标不是把敏感证件原图放进代码库，而是：

1. 在仓库内保存 repo-safe 的样本清单与结果格式
2. 用统一脚本计算可复现的 OCR 基线摘要
3. 固定后续 provider 切换时必须对比的指标口径

## 2. 资产清单

本轮新增资产如下：

1. 命令行评估器：`go run ./cmd/ocr_baseline_eval`
2. 样本清单示例：`ocr/testdata/ocr_baseline_manifest.example.json`
3. 运行结果示例：`ocr/testdata/ocr_baseline_run.example.json`
4. Makefile 入口：`make ocr-baseline manifest=... report=... [out=...]`

## 3. 样本集原则

### 3.1 仓库内只保存清单，不保存敏感原图

身份证、营业执照、健康证等真实图片不应直接提交到仓库。

仓库内保存：

1. 样本 ID
2. document_type / owner_type / side
3. 场景标签，例如 `clean_scan`、`slight_glare`、`cropped_edge`
4. 预期结构化字段

真实图片建议存放在受控目录或对象存储中，并通过样本 ID 与清单做映射。

### 3.2 最低覆盖面

每个 document type 至少应覆盖：

1. 一组清晰样本
2. 一组轻度模糊/反光/裁边样本
3. 一组 provider 已知易错样本
4. 一组需要人工介入的失败样本

建议首版最小样本量：

1. `business_license` ≥ 20
2. `id_card` front/back 合计 ≥ 20
3. `food_permit` ≥ 20
4. `health_cert` ≥ 20

## 4. 评估输入格式

### 4.1 manifest

manifest 是基线清单，字段包括：

1. `dataset_name`
2. `version`
3. `samples[]`

每个 sample 至少包含：

1. `sample_id`
2. `document_type`
3. `owner_type`
4. `side`
5. `scenario`
6. `expected_fields`

### 4.2 run report

run report 是一次 provider 跑批结果，字段包括：

1. `provider`
2. `generated_at`
3. `queue_snapshot.pending`
4. `queue_snapshot.processing`
5. `samples[]`

每个 sample result 至少包含：

1. `sample_id`
2. `status`
3. `error_code`
4. `attempt_count`
5. `latency_ms`
6. `recognized_fields`

## 5. 指标口径

以下口径固定为 OCR 基线输出，后续 provider 对比必须保持一致。

### 5.1 成功率

$$
success\_rate = \frac{succeeded\_samples}{total\_samples}
$$

说明：

1. 分母固定使用 manifest 样本总数
2. 缺失结果样本仍计入分母

### 5.2 字段准确率

$$
field\_accuracy = \frac{matched\_expected\_fields}{total\_expected\_fields}
$$

当前脚本的字段比较规则固定为：

1. 转小写
2. 去掉空白
3. 去掉标点和符号
4. 再做逐字段完全匹配

### 5.3 耗时

耗时统计使用已完成样本的 `latency_ms`，即 `status in {succeeded, failed, cancelled}`。

固定输出：

1. `P50`
2. `P95`
3. `P99`

### 5.4 失败码分布

统计所有非空 `error_code` 的出现次数，输出 map。

### 5.5 堆积量

$$
backlog\_count = queue\_snapshot.pending + queue\_snapshot.processing
$$

### 5.6 重试量

$$
retry\_volume = \sum max(attempt\_count - 1, 0)
$$

## 6. 使用方式

直接运行：

```bash
go run ./cmd/ocr_baseline_eval \
  -manifest ocr/testdata/ocr_baseline_manifest.example.json \
  -run ocr/testdata/ocr_baseline_run.example.json
```

或通过 Makefile：

```bash
make ocr-baseline \
  manifest=ocr/testdata/ocr_baseline_manifest.example.json \
  report=ocr/testdata/ocr_baseline_run.example.json
```

写入文件：

```bash
make ocr-baseline \
  manifest=/secure/ocr/manifest.json \
  report=/secure/ocr/run.json \
  out=/tmp/ocr-baseline-summary.json
```

## 7. 输出摘要

脚本会输出：

1. 总样本数
2. 完成数 / 成功数 / 缺失数
3. success_rate
4. field_accuracy
5. retry_volume
6. backlog_count
7. latency_ms.p50 / p95 / p99
8. error_code_distribution
9. per_document_type 摘要

## 8. 后续使用要求

后续任何 provider 路由调整、字段解析规则修改或大规模清洗逻辑调整前后，都必须至少执行一次基线对比，并回答以下问题：

1. 总成功率是否提升或持平
2. `id_card`、`business_license`、`food_permit`、`health_cert` 的字段准确率是否退化
3. P95/P99 是否显著恶化
4. 是否引入新的高频 `error_code`
5. 重试量和堆积量是否异常抬升

## 9. 限制说明

当前脚本定位是基线比较器，不负责：

1. 直接调用 provider 跑 OCR
2. 管理敏感原图存储
3. 自动生成 run report

这些动作应在受控测试环境中完成，再把结果导出为 run report 交给本脚本计算。