# v-platform-backend 审查报告 — 对标 LinkFox 类 SaaS 平台

> **日期**: 2026-04-21
> **目的**: 评估 v-platform-backend 现有能力对标 LinkFox 类跨境电商 AI 运营平台的匹配度，明确已具备能力、核心差距和补齐路径。
> **参照物**: [linkfox.com](https://www.linkfox.com) — 跨境电商 AI 工具平台（AI 作图 + Agent 对话 + 素材管理 + 团队协作 + 算力积分计费）

---

## 一、已审查范围

| 文件/目录 | 说明 |
|---|---|
| `internal/models/core.go` | User, Organization, OrganizationMember, Permission, Role, RolePermission |
| `internal/models/commercial.go` | Product, SKU, CommercialPackage, BillableItem, RateCard, CommercialEntity, MerchantAccount, SettlementAccount, BillingProfile, RoutingPolicy, OrgBillingProfile, MeterEvent, UsageRecord, UsageAgg |
| `internal/models/commercial.go` + `internal/models/finance.go` | QuotaLedger, historical CreditsLedger compatibility model, BillingLedger, ResourceReservation, WalletAccount, WalletBucket, WalletLedger |
| `internal/models/finance.go` | WalletAccount, AssetDefinition, WalletBucket, AllowancePolicy, WalletLedger, SettlementRecord, DiscountLedger, RewardLedger, CommissionLedger, ReferralProgram, ReferralCode, ReferralConversion |
| `internal/models/channel_finance.go` | ChannelPartner 系列 12+ 模型 |
| `internal/models/audit.go` | AuditLog |
| `internal/modules/identity/service.go` | 注册/登录/JWT 逻辑 |
| `internal/modules/access/seed.go` | 内置角色与权限种子数据 |
| `internal/modules/metering/service.go` | 计量结算核心逻辑 |
| `internal/modules/wallet/service.go` | 钱包/资产生命周期管理 |
| `docs/BACKEND_GUIDE.md` | 项目入口指南 |
| `docs/architecture/SERVICE_BOUNDARY.md` | 服务边界定义 |
| `docs/architecture/COMMERCIALIZATION_BOUNDARY.md` | 商业化边界与六阶段路径 |

---

## 二、已具备能力清单

### 2.1 数据模型统计

- **Core 模型**: 6 个（User, Organization, OrganizationMember, Permission, Role, RolePermission）
- **Commercial/Catalog 模型**: 14 个
- **Finance/Wallet 模型**: 12 个
- **Control 模型**: 4 个
- **Channel 模型**: 12+ 个
- **Audit 模型**: 1 个
- **总计**: ~42 个数据模型

### 2.2 API 路由统计

| 分组 | Public API | Internal API |
|---|---|---|
| Auth/Identity | 3 | 2 |
| Organization | 2 | 1 |
| Access/Permission | 1 | 1 |
| Catalog | 12 | 0 |
| Commercial/Routing | 7 | 1 |
| Controls (Quota/Credits) | 6 | 3 |
| Wallet | 13 | 11 |
| Incentives | 28+ | 28+ |
| Metering | 6 | 6 |
| Docs/Health | 4 | 0 |
| **总计** | **~85** | **~55** |

### 2.3 已具备的核心优势

| 能力域 | 对标 LinkFox 需求 | 评估 |
|---|---|---|
| 计量-结算闭环 | 算力点数扣减 | 完整：MeterEvent → UsageRecord → Settlement → Ledger，支持 quota / credits / usage_billing / included_then_overage 四种结算模式，含冲正 |
| 钱包/积分 | 充值、扣减、过期、周期重置 | 完整：WalletAccount + WalletBucket + AllowancePolicy + 自动过期调度 + 优先级扣减 |
| 配额管理 | 套餐内包含量 | 完整：QuotaLedger + ResourceReservation（预留/提交/释放） |
| 组织模型 | 团队/企业 | 基本具备：Organization + OrganizationMember + 多组织切换 + JWT 含 org 上下文 |
| 权限系统 | 角色权限控制 | 基本具备：4 个内置角色（owner/admin/developer/viewer）+ 10 个权限 + 中间件校验 |
| 商业路由 | 多主体/多地区 | 完整：CommercialEntity → BillingProfile → RoutingPolicy 多级路由 |
| 渠道分佣 | 渠道合作伙伴 | 超配：12 个模型的完整分佣体系（LinkFox 不具备此复杂度） |
| 审计日志 | 操作记录追溯 | 完整：diff-capable AuditLog（含 before_snapshot / after_snapshot / diff_summary） |
| 可观测性 | 运维监控 | 完整：OTel Tracing + Prometheus Metrics + 结构化日志 + HMAC 内部服务认证 |

---

## 三、核心差距分析

### 3.1 用户系统差距

| 能力项 | LinkFox 典型 | 当前状态 | 差距等级 |
|---|---|---|---|
| Email+密码注册/登录 | 有 | 有 | 无 |
| 手机号注册/登录 | 有 | 无（User 模型无 phone 字段） | 缺失 |
| 第三方 OAuth（Google/微信/GitHub） | 有 | 无（无 OAuth Provider 模型） | 缺失 |
| 用户头像/昵称修改 | 有 | 仅内部 API（无公开 API） | 半缺失 |
| 密码重置/找回 | 有 | 无 | 缺失 |
| 邮箱验证 | 有 | 无（注册无验证流程） | 缺失 |
| 账户封禁/冻结/注销 | 有 | 仅 status 字段，无管理 API | 半缺失 |
| 用户搜索/列表（管理端） | 有 | 无 | 缺失 |
| 多设备登录管理/Token 吊销 | 有 | 无 | 缺失 |

### 3.2 组织/团队管理差距

| 能力项 | LinkFox 典型 | 当前状态 | 差距等级 |
|---|---|---|---|
| 创建组织 | 有 | 仅注册时自动创建，无独立接口 | 缺失 |
| 组织信息编辑 | 有 | 仅内部 API | 半缺失 |
| 邀请成员（邮件/链接） | 有 | 无 | 缺失 |
| 移除成员 | 有 | 无 | 缺失 |
| 修改成员角色 | 有 | 无 | 缺失 |
| 成员列表 | 有 | 无 | 缺失 |
| 子团队/部门 | 有 | 无（无 Team 模型） | 缺失 |
| 组织切换 | 有 | 有 | 无 |
| 多组织归属 | 有 | 有（通过 membership） | 无 |
| 组织级配额/套餐 | 有 | 有（Plan + OrgBillingProfile） | 基本具备 |

### 3.3 订单/支付/订阅差距

| 能力项 | LinkFox 典型 | 当前状态 | 差距等级 |
|---|---|---|---|
| 订单模型（Order） | 有 | 无（文档规划在第六阶段） | 缺失 |
| 支付集成（Stripe/微信/支付宝） | 有 | 无（仅有 MerchantAccount 模型定义） | 缺失 |
| 订阅生命周期 | 有 | 无（无 Subscription 模型） | 缺失 |
| 退款流程 | 有 | 无（规划在第六阶段） | 缺失 |
| 发票生成 | 有 | 无（规划在第五阶段） | 缺失 |
| 价格策略（阶梯/按量） | 有 | 仅 flat 单价 | 弱 |
| 优惠券/促销码 | 有 | 仅 DiscountLedger，无 Coupon 实体 | 半缺失 |
| 试用期管理 | 有 | 无 | 缺失 |
| 套餐升降级 | 有 | 无 | 缺失 |

### 3.4 积分/钱包差距（已基本具备）

| 能力项 | LinkFox 典型 | 当前状态 | 差距等级 |
|---|---|---|---|
| 钱包余额/充值/扣减 | 有 | 有 | 无 |
| 积分/Credits | 有 | 有 | 无 |
| 配额管理 | 有 | 有 | 无 |
| 过期管理 | 有 | 有（WalletBucket + 自动过期） | 无 |
| 周期重置 | 有 | 有（AllowancePolicy + 自动调度） | 无 |
| 多资产类型 | 有 | 有（AssetDefinition） | 无 |
| 预留/冻结/释放 | 高级 | 有（ResourceReservation） | 无 |

### 3.5 素材/内容管理差距（完全缺失，属产品侧）

| 能力项 | 说明 | 建议归属 |
|---|---|---|
| 素材上传/存储 | OSS/S3 适配 | 平台提供 OSS 适配层 |
| 素材分类/标签/搜索 | 多维标签、全文搜索 | 产品后端 |
| 素材关系网络 | 衍生/变体/引用/版本 | 产品后端 |
| 素材版本管理 | 精修/重生成的版本链 | 产品后端 |
| 品牌库 | 品牌 DNA + 敏感词 | 产品后端 |
| 知识库（RAG） | 文档解析 + 向量检索 | 产品后端 |
| 模板市场 | Prompt 模板 CRUD + 推荐 | 产品后端 |

---

## 四、个人 ↔ 团队身份切换设计方案

### 4.1 设计原则

1. **身份与资产分离** — 用户切换身份不改变资产归属
2. **组织是资产容器** — 所有资产都挂在某个 Organization 下
3. **个人 = 一人组织** — 注册时自动创建的就是"个人组织"（当前已实现）
4. **切换 = 上下文切换** — 不是数据迁移，是视角切换

### 4.2 架构设计

```
User (用户)
├── current_org_id ← 当前活跃组织（已有字段）
│
├── OrgMembership(org=personal_org, role=owner)
│   └── Organization(type=personal)
│       ├── 我的素材 / 我的模板 / 我的配额 / 我的钱包 / 我的记录
│
├── OrgMembership(org=team_A, role=admin)
│   └── Organization(type=team)
│       ├── 团队素材 / 共享品牌库 / 团队配额 / 团队钱包 / 团队记录
│
└── OrgMembership(org=team_B, role=viewer)
    └── Organization(type=team)
        ├── (只读访问)
```

### 4.3 切换场景处理

#### 个人 → 创建/加入团队

1. 创建 Organization(type=team, plan=team)
2. 创建 OrgMembership(user=me, role=owner)
3. 个人组织保留（不删除）
4. 可选迁移部分素材到团队（copy, 非 move）
5. 邀请成员加入
6. 结果：用户同时拥有个人空间 + 团队空间，随时切换

#### 团队成员退出

1. 删除 OrgMembership
2. 个人组织不受影响
3. 团队内生成的素材归属团队（owner_id=team_org）
4. 个人上传的素材归属个人（owner_id=personal_org）
5. current_org_id 自动切回个人组织

#### 团队解散

1. 移除所有其他成员
2. 团队素材可批量迁移到 owner 个人组织
3. 团队配额/钱包余额按策略处理（转入个人 or 退款）
4. Organization.status = archived

### 4.4 各维度差异处理规则

| 维度 | 个人模式 | 团队模式 | 切换规则 |
|---|---|---|---|
| 配额 | 绑定个人 Org | 绑定团队 Org | 各自独立，互不影响 |
| 钱包 | WalletAccount(subject=personal_org) | WalletAccount(subject=team_org) | 各自独立 |
| 素材 | Material(owner=personal_org) | Material(owner=team_org) | 按 visibility 控制可见性 |
| 生成记录 | 挂在个人 org | 挂在团队 org | 切换时看到不同记录 |
| 品牌库 | 个人私有 | 团队共享 | 切换时加载不同品牌 |
| 知识库 | 个人文档 | 团队文档 | 切换时搜索不同文档 |
| 计费 | 扣个人配额/钱包 | 扣团队配额/钱包 | JWT 中 org_id 决定扣谁 |
| 操作权限 | 全部权限（owner） | 按角色控制 | 中间件根据 org_role 判断 |

---

## 五、素材体系设计方案

### 5.1 素材数据模型建议（产品后端）

```
Material (素材基础)
├── id, type(image/document/template/brand_asset/scene_ref)
├── owner_type(platform/org/user), owner_id
├── category_tags[], scene_tags[], platform_tags[]
├── status(draft/active/archived)
├── visibility(public/org/private)
├── source(uploaded/generated/crawled/imported)
├── usage_count, favorite_count, quality_score
├── metadata(JSON): 尺寸/格式/色调/风格/模型参数
├── parent_id (衍生关系：由哪个素材生成)
├── created_at, updated_at

MaterialRelation (素材关系)
├── source_id, target_id
├── relation_type(derived_from/variant_of/reference/bundled_with/version_of)
├── metadata

MaterialCollection (素材集/套组)
├── id, name, type(brand_kit/listing_set/campaign)
├── owner_type, owner_id
├── items[] (ordered)

MaterialVersion (版本管理)
├── material_id, version, file_url
├── change_note, created_at
```

### 5.2 关系维护策略

| 关系类型 | 自动化程度 | 实现方式 |
|---|---|---|
| derived_from | 全自动 | AI 工具生成时自动记录 input → output |
| variant_of | 全自动 | 同一输入不同参数生成的变体 |
| reference | 半自动 | 使用场景参考图时自动记录；用户可手动关联 |
| bundled_with | 全自动 | 套组生成（如 Listing 套图）自动打包 |
| version_of | 全自动 | 精修/重新生成时自动建立版本链 |

### 5.3 衍生能力

| 能力 | 依赖关系 |
|---|---|
| 一键重新生成全套 | bundled_with → 知道套组成员 → 批量重做 |
| 风格迁移 | reference → 知道参考场景 → 换场景重生成 |
| AB 测试素材 | variant_of → 不同变体对比 → 选最优 |
| 素材溯源 | derived_from 链条 → 追溯到原始输入 |
| 智能推荐 | 协同过滤 → 用了 A 的人也用了 B |
| 品牌一致性检查 | brand_kit + generated → 检测偏离 |

---

## 六、补齐路径（按优先级排序）

### P0 — 产品上线前必须（~2 周）

| 改动项 | 工作量 | 说明 |
|---|---|---|
| Organization 加 `type` 字段 | 0.5 天 | `personal` / `team` / `enterprise` |
| 创建组织 API | 1 天 | `POST /api/v1/orgs` |
| 邀请/移除成员 API | 2 天 | invite + remove + 邮件/链接邀请 |
| 成员列表/角色修改 API | 1 天 | list + patch role |
| 公开用户管理 API | 1-2 天 | 修改密码、修改个人信息、上传头像 |
| 密码重置流程 | 1 天 | 邮件验证码 + 重置接口 |

### P1 — 商业化闭环（~3 周）

| 改动项 | 工作量 | 说明 |
|---|---|---|
| 订单模型（Order） | 3-5 天 | 按 COMMERCIALIZATION_BOUNDARY 第六阶段推进 |
| 支付集成 | 5-7 天 | 至少接入一个支付渠道（Stripe / 微信） |
| 订阅生命周期 | 3-5 天 | Subscription + billing cycle engine |
| 套餐升降级逻辑 | 2-3 天 | 含 prorate 按比例退补 |

### P2 — 用户增长 & 营销（~2 周）

| 改动项 | 工作量 | 说明 |
|---|---|---|
| 手机号登录 | 2-3 天 | User 加 phone 字段 + SMS 验证码 |
| OAuth 登录 | 3-5 天 | Google / 微信 / GitHub Provider |
| 优惠券/促销码 | 2-3 天 | 补 Coupon 实体 + 核销逻辑 |
| 邮箱验证流程 | 1-2 天 | 注册后邮箱确认 |

### P3 — 精细化运营（~2 周）

| 改动项 | 工作量 | 说明 |
|---|---|---|
| 自定义角色 API | 2-3 天 | Role CRUD |
| 价格模型扩展 | 2-3 天 | RateCard 支持阶梯定价 |
| 发票/账单 | 3-5 天 | 按第五阶段推进 |
| 试用期管理 | 1-2 天 | 含到期自动降级 |

### 产品侧（非平台层）

| 改动项 | 归属 | 说明 |
|---|---|---|
| 素材管理系统 | 产品后端 | Material + MaterialRelation + OSS 存储 |
| AI 工具调度引擎 | 产品后端 | 模型调用、队列、结果回调 |
| 模板市场 | 产品后端 | Prompt 模板 CRUD + 分类 + 推荐 |
| 知识库（RAG） | 产品后端 | 文档解析、向量化、检索 |
| 场景参考图库 | 产品后端 | 平台供给 + UGC |
| 品牌库 | 产品后端 | 品牌 DNA + 敏感词管理 |

---

## 七、关键结论

1. **商业化底座已是行业高水准**：计量 → 结算 → 钱包 → 台账的完整闭环，加上渠道分佣体系，远超 LinkFox 这类产品的底层需求。
2. **"最后一公里"差距在用户自服务和团队协作**：成员管理、密码重置、个人信息修改这些基础 API 缺失，是产品上线的硬性阻碍。
3. **个人↔团队切换架构已天然支持**：Organization + OrgMembership + org switch + JWT org 上下文的设计只需加 `org_type` 字段即可完整支持。
4. **素材系统是产品核心但非平台职责**：平台提供 OSS 适配层和计量计费接口，素材业务逻辑（分类/关系/版本）归产品后端。
5. **订单/支付是商业化闭环的最后一环**：按已规划的六阶段路径稳步推进即可。
