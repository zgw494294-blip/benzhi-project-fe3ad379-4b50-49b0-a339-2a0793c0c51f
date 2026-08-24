# 传感器校准放行服务

本项目面向环境监测站运维团队，将传感器部署前的建批登记、标准点方案锁定、多点重复采样、自动误差判定、返校复验、独立质量复核和部署凭据签发纳入一条可追溯流程。服务提供 JSON HTTP API，并将业务事件写入带摘要链的本地 JSON Lines 账本；投影快照使用 `Sync` 和原子 `Rename` 更新，启动时会验证账本并在需要时重建快照。

## 构建与测试

项目要求 Go 1.22 或更高版本。

```bash
go build ./...
go test ./...
```

## 运行服务

默认监听高位回环地址 `127.0.0.1:19081`，默认数据目录为 `./data`：

```bash
go run ./cmd/server
```

可通过 `-addr` 指定完整监听地址，通过 `-data` 指定持久化目录：

```bash
go run ./cmd/server -addr=127.0.0.1:19120 -data=./runtime-data
```

也可以设置 `PORT`，服务会监听 `127.0.0.1:<PORT>`。显式传入 `-addr` 时以命令行参数为准。服务不会默认绑定 `0.0.0.0`。

## 有界 selfcheck

以下命令会在真实 TCP 监听地址上启动 HTTP 服务，依次完成不合格采样、批量读数、幂等重放、版本冲突、返校复验任务查询、独立审核、冻结、凭据签发、主动核验和工作队列查询，然后主动关闭服务并退出：

```bash
go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck -selfcheck-timeout=15s
```

selfcheck 未指定 `-data` 时使用临时目录并在结束后清理，不会污染正式账本。

## API 流程

所有变更请求都显式携带 `actor`、`expectedVersion` 和 `idempotencyKey`。版本不匹配返回 `409 conflict`；同一批次与同一命令的相同 `idempotencyKey` 会安全返回原结果；同一个键用于其他命令会被拒绝。

主要端点如下：

- `GET /healthz`：检查服务与账本完整性。
- `POST /v1/batches`：创建校准批次。
- `GET /v1/batches`：按 `stationCode`、`status`、`createdFrom`、`createdTo` 筛选工作队列，并用 `limit`、`cursor` 稳定分页；响应同时返回各状态和待处理汇总。
- `GET /v1/batches/{batchID}`：查询批次、当前修订、方案、读数、复验覆盖、问题和凭据。
- `POST /v1/batches/{batchID}/sensors`：登记传感器身份与量程。
- `POST /v1/batches/{batchID}/profile:lock`：验证覆盖范围并锁定标准点方案。
- `POST /v1/batches/{batchID}/measurements`：提交指定修订和标准点的不可变重复读数。
- `POST /v1/batches/{batchID}/measurements:batch`：为同一当前修订原子提交多个标准点的不可变重复读数，正文使用 `measurements` 数组。
- `POST /v1/batches/{batchID}/recalibrations`：为存在问题的传感器建立返校修订。
- `GET /v1/batches/{batchID}/recalibrations/{revisionID}/tasks`：查询当前返校修订的必测、通过、仍不合格及继承任务。
- `POST /v1/batches/{batchID}/reviews`：由独立复核员提交 `approve` 或 `return` 决定；退回时 `corrections` 至少包含一项结构化补正要求。
- `POST /v1/batches/{batchID}/reviews:resubmit`：在所有自动及人工问题闭环、有效证据完整后再次送审。
- `POST /v1/batches/{batchID}/release`：为已冻结批次签发部署放行凭据。
- `GET /v1/batches/{batchID}/credential`：查询并现场验证凭据摘要。
- `POST /v1/batches/{batchID}/credential:verify`：使用 `credentialID` 和部署方收到的 `contentDigest` 只读核验账本、冻结投影和设备修订清单。
- `GET /v1/batches/{batchID}/findings`：查询未闭环问题。
- `GET /v1/batches/{batchID}/audit`：按账本顺序查询审计轨迹。

读数完整后，系统计算均值、绝对误差、相对误差和极差。批量提交先校验所有标准点，任一项目无效时不保存任何读数，也不推进版本。返校修订必须用当前修订的新证据覆盖原问题标准点；其他已合格标准点可以沿用旧修订证据，并在批次详情的 `coverage` 和复验任务中标明来源。结构化退回产生人工问题项，补正完成后必须显式再次送审。复核员不能是该批次的任何采样提交人。审核通过即冻结校准内容，之后只能签发一次不可变凭据。

## 持久化文件

指定数据目录中包含：

- `events.jsonl`：带 `schemaVersion`、递增 `sequence`、`previousDigest` 和 `digest` 的事件账本，每个事件携带可重放投影。
- `snapshot.json`：当前聚合投影及对应账本序号和末尾摘要。

服务启动时验证完整摘要链、每个事件投影的领域关联以及快照位置。快照缺失、损坏或落后时会从最后一个有效事件重建；账本被截断、乱序或篡改时服务拒绝启动。
