# 商业化商品模型设计

## 1. 目标

本文档定义平台商业化体系中的商品基础模型，解决以下问题：

- 平台如何统一表达“卖什么”
- 产品如何把业务动作映射到可计费对象
- entitlement、metering、billing 如何共享同一套商业语言

本文档只定义共享模型，不定义某个产品自己的营销文案或页面展示。

## 2. 设计原则

- 平台只保存通用商业抽象，不保存产品特有业务术语
- 同一个产品可以有多个售卖形态，但底层能力模型应统一
- 计量对象、售卖对象、权益对象要拆开，不混成一个字段
- 后续 billing、entitlement、quota、credits 都必须复用这套模型

## 3. 核心对象

### 3.1 `product`

表示产品线，是最上层业务归属。

示例：

- `menu_ai`
- `kyc`
- `attendance`

建议字段：

- `id`
- `code`
- `name`
- `status`
- `owner_team`
- `metadata`
- `created_at`
- `updated_at`

说明：

- `product` 用于表达“哪个产品”
- 不直接承担计费逻辑

### 3.2 `sku`

表示可售卖的标准商业单元，是 billing 和销售口径里的主对象。

示例：

- `menu_pro_monthly`
- `menu_credit_pack_1000`
- `kyc_ocr_payg`

建议字段：

- `id`
- `product_id`
- `code`
- `name`
- `sku_type`
- `billing_mode`
- `status`
- `currency`
- `list_price`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `sku_type`: `subscription`, `package`, `add_on`, `payg`
- `billing_mode`: `recurring`, `one_time`, `usage_based`, `hybrid`

说明：

- `sku` 是平台销售和 billing 的主锚点
- 一个产品可以有多个 sku
- 一个 sku 可以绑定多个 entitlement 和多个 billable item

### 3.3 `package`

表示“权益包”或“能力包”，用于表达买到这个东西后，用户理论上能用哪些能力。

示例：

- `menu_pro_capability_pack`
- `kyc_growth_bundle`

建议字段：

- `id`
- `product_id`
- `code`
- `name`
- `package_type`
- `status`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `package_type`: `capability_bundle`, `resource_bundle`, `hybrid`

说明：

- `package` 偏权益表达
- 不直接承担价格和支付
- entitlement 模块主要消费这个对象

### 3.4 `billable_item`

表示可计量、可计费、可统计的“最小收费项”。

这是 metering、usage、billing 的核心对象。

示例：

- `menu_generate_image`
- `menu_publish_menu`
- `kyc_ocr_request`
- `kyc_liveness_second`

建议字段：

- `id`
- `product_id`
- `code`
- `name`
- `meter_unit`
- `billing_scope`
- `settlement_mode`
- `pricing_behavior`
- `status`
- `metadata`
- `created_at`
- `updated_at`

建议枚举：

- `meter_unit`: `request`, `second`, `image`, `document`, `credit`
- `billing_scope`: `org`, `user`, `workspace`
- `settlement_mode`: `quota`, `credits`, `usage_billing`, `included_then_overage`
- `pricing_behavior`: `per_call`, `bundle_entry`, `child_non_billable`, `hybrid`

说明：

- 一个业务动作最终应映射到一个或多个 `billable_item`
- billing、usage statement、ledger 都围绕它展开
- `billing_scope` 用于定义到底按组织、按用户还是按工作空间计量
- `pricing_behavior` 用于表达单次计费还是打包入口计费

### 3.5 `rate_card`

表示价格规则，是把 billable item 或 sku 与价格口径绑定起来的配置对象。

示例：

- `menu_generate_image_cn_cny_v1`
- `kyc_ocr_usd_payg_v2`

建议字段：

- `id`
- `product_id`
- `code`
- `target_type`
- `target_id`
- `price_model`
- `currency`
- `price_config`
- `effective_from`
- `effective_to`
- `status`
- `version`
- `created_at`
- `updated_at`

建议枚举：

- `target_type`: `sku`, `billable_item`
- `price_model`: `flat`, `tiered`, `volume`, `package_included`, `hybrid`

说明：

- `rate_card` 只负责价格口径
- `rate_card` 不替代 package，也不替代 sku
- 后续多地区、多主体场景下，可以按 `billing_profile` 选择不同 `rate_card`

## 4. 对象关系

建议关系如下：

1. 一个 `product` 有多个 `sku`
2. 一个 `product` 有多个 `package`
3. 一个 `product` 有多个 `billable_item`
4. 一个 `sku` 可以关联多个 `package`
5. 一个 `package` 可以包含多个 entitlement 项
6. 一个 `sku` 可以映射一个或多个 `rate_card`
7. 一个 `billable_item` 可以映射一个或多个 `rate_card`

建议增加关联表：

- `sku_packages`
- `sku_billable_items`
- `package_entitlements`

## 5. 为什么要拆成这 5 个对象

### 5.1 不能只用一个套餐表

如果把售卖、权益、计量、价格全部塞进一个“套餐表”，后面会出现这些问题：

- 一个套餐既要表达销售，又要表达能力，又要表达价格，语义混乱
- 按量计费和订阅制很难统一
- 产品后续要做 add-on、资源包、超额收费时会非常别扭

### 5.2 `sku` 和 `billable_item` 不能混

因为它们关注点不同：

- `sku` 是卖给客户看的
- `billable_item` 是系统内部计量和计费的最小单位

例如：

- 用户买的是 `menu_pro_monthly`
- 实际计量的可能是 `menu_generate_image`
- 超额计费的可能又是 `menu_generate_image_overage`

### 5.3 `package` 和 `rate_card` 也不能混

因为：

- `package` 解决“买完能用什么”
- `rate_card` 解决“怎么收钱”

同一个 package 在不同地区、不同主体、不同渠道下，价格可能完全不同。

## 6. 推荐的第一版数据表

建议第一版优先建设这些表：

- `products`
- `skus`
- `packages`
- `billable_items`
- `rate_cards`
- `sku_packages`
- `sku_billable_items`
- `package_entitlements`

第一版可以先不急着做复杂版本控制，但要预留：

- `status`
- `version`
- `effective_from`
- `effective_to`

## 7. 与后续模块的依赖关系

### 7.1 entitlement 依赖

entitlement 主要依赖：

- `package`
- `package_entitlements`
- `sku_packages`

### 7.2 metering 依赖

metering 主要依赖：

- `billable_item`
- `sku_billable_items`

### 7.3 billing 依赖

billing 主要依赖：

- `rate_card`
- `billable_item`
- `sku`

### 7.4 多主体路由依赖

多主体路由主要会影响：

- 一个订单选用哪个 `billing_profile`
- 该 `billing_profile` 下应该应用哪个 `rate_card`

## 8. Menu 参考映射

以 Menu AI 为例，可先这样映射：

- `product`: `menu_ai`
- `sku`: `menu_pro_monthly`, `menu_credit_pack_1000`
- `package`: `menu_pro_capability_pack`
- `billable_item`: `menu_generate_image`, `menu_publish_menu`
- `rate_card`: `menu_generate_image_cny_v1`

这意味着：

- 销售侧卖的是 `sku`
- 权益侧开的是 `package`
- 计量侧记的是 `billable_item`
- 价格侧套的是 `rate_card`

### 8.1 Menu 当前诉求映射检查

结合 `AI_Menu_Growth_Engine_Frontend/docs/FRONTEND_TECH_DOC.md` 当前诉求，这套模型基本能支撑：

- `Free / Pro / Growth` 对应不同 `sku`
- 月度 credits 对应 `package + entitlement + credits ledger`
- `AI 图片增强 / 导出 / QR 菜单生成` 对应不同 `billable_item`
- `AI 文案生成免费` 对应 `billable_item` 存在但 `billable=false` 或 `rate_card=0`
- 订阅套餐 + 按次扣点并存，对应 `billing_mode=hybrid`

但有一个边界要尽早定死：

- 前端文档当前把 `credits` 挂在 `User` 上
- 平台侧建议默认把 credits、quota、billing 归到 `org` 或 `workspace`

原因：

- 商业化结算主体通常不是单个用户，而是组织或店铺主体
- 后续店铺多人协作、代理商、多门店时，用户级余额会很难演进

因此建议：

- Menu 显示层可以继续展示“我的 credits”
- 底层结算模型优先按 `org` 或 `workspace` 记账

### 8.2 KYC 当前诉求映射检查

对于 KYC：

- “不同能力按 API 统计计费”可以用一个 API 对应一个 `billable_item`
- “多个 API 打包计费”需要结合 `pricing_behavior` 与 metering 文档中的 `charge_group_id`

推荐映射：

- `kyc_ocr_request` -> `per_call`
- `kyc_face_verify_request` -> `per_call`
- `kyc_bundle_entry` -> `bundle_entry`
- `kyc_bundle_child_ocr` / `kyc_bundle_child_liveness` -> `child_non_billable`

## 9. 第一版落地建议

第一版先做“够用且稳定”的能力，不追求一步到位：

1. 先把对象和字段定义清楚
2. 先做关联关系和基础管理接口
3. 暂不做复杂商品编排器
4. 暂不做特别复杂的阶梯价编辑器
5. 先保证 entitlement、metering、billing 能共享同一套模型

## 10. DoD

这一模块进入可实施状态的标准：

- 对象定义清晰，无明显语义重叠
- `sku / package / billable_item / rate_card` 关系可落库
- 能支撑至少一个 Menu 场景和一个 KYC 场景
- entitlement、metering、billing 三个方向都能引用这套模型
- 管理变更可审计
