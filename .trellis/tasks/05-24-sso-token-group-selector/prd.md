---
task: sso-token-group-selector
parent_task: productflow-sso
design_session: 2026-05-24 grill-me
scope_tier: A
estimated_days: 0.5
file_count: 3
cross_repo: false
---

# ProductFlow SSO Token Group Selector

## Goal

替换 new-api `/system-settings/operations/productflow-sso` 配置页面的 **Token group** 字段:
将当前的手填 `<Input>` 改为单选 `<Select>` 下拉, 选项数据从已存在的 `GET /api/group/`
接口拉取 (即"分组倍率"面板里 admin 已经定义的所有分组名).

消除一类失效场景: admin 手填一个 GroupRatio 里没定义的分组名 (如拼错 `productfllow`),
保存时不报错, ProductFlow 跑起来才返回 "no available channel".

## Why Now

当前实现 ([productflow-sso-settings-section.tsx:511-516](../../../web/default/src/features/system-settings/integrations/productflow-sso-settings-section.tsx)):

```tsx
<Input placeholder={t('image')} {...field} />
```

- 无校验, 无补全, 无空值提示
- admin 不知道有哪些分组可选, 也看不到自己拼错没拼错
- 默认值 placeholder 是 `image`, 但 GroupRatio 实际默认含 `default / vip / svip`
  ([group_ratio.go:12-16](../../../setting/ratio_setting/group_ratio.go)), 误导
- ProductFlow 公网部署后,这类静默失效问题排查路径长 (用户报告 → 看 ProductFlow 日志 →
  看 new-api 日志 → 才能定位到"SSO 配置写错了分组名")

下拉选项数据源已经存在: `GET /api/group/` ([controller/group.go:14-24](../../../controller/group.go))
返回 `string[]`, 内部就是从 `ratio_setting.GetGroupRatioCopy()` 取的——即"分组倍率"
面板维护的同一份真理来源.

## Decision Matrix (from 2026-05-24 grill-me + r1 review)

下表是本任务"决定做什么 / 不做什么"的权威记录, reviewer 应根据本表挑战具体决策点
而不是重跑 grill 流程.

| # | Decision Point | Resolution | Why |
|---|----------------|------------|-----|
| 1 | 接口形态 | **复用现有 `GET /api/group/` 返回 `string[]`**, 后端零改动 | 现有接口已有 3 处消费者 (users / subscriptions / classic 前端); 升级返回结构需连改 4 文件超过文件上限; 新增详情接口长期维护负担; 当前 task 只需名字列表, `string[]` 够用 |
| 2 | 下拉项展示形态 | **纯分组名**, 不带渠道数 / 倍率 / 描述 | "渠道数 / 倍率 / 描述"分别是渠道页 / 计费页 / 用户面板的关注点; SSO 配置面板只负责"选哪个分组", 越界即职责泄漏 |
| 3 | 选错空壳分组的防呆 | **不防御** | 防呆需引入 channel_count 等非本面板关心的数据; 选错的发现路径 (ProductFlow 实跑报错) 短而清晰; admin 在 SSO 面板看到的"看似可选"分组都是 GroupRatio 已定义的, 不会进一步错位 |
| 4 | 多选 vs 单选 | **单选** | DB 里 `token.Group` 是单值字段 ([productflow_sso_token.go:96](../../../controller/productflow_sso_token.go)), 多选会让前端 `"a,b"` 字符串塞进单字段后调度匹配不到任何"分组名叫 a,b 的渠道"——静默失效 |
| 5 | ➕新建分组快捷入口 | **无快捷入口**, 空数据态显示文字"请去 系统设置 → 模型与倍率 → 分组倍率 创建" | 跳转链接需路由级守卫防草稿丢失; 新 tab 体验在 admin 工具里反模式; inline 新建违反职责边界; 配 SSO 是低频操作 |
| 6 | 孤儿分组健康检查 | **不做, 进 backlog (BL-1)** | channel.Group 贴了但 GroupRatio 没定义的隐患存在但跟 SSO 面板无关; 应在"分组倍率"面板做扫描提示 |
| 7 | i18n 缺翻译 | **不进本任务, admin 重新部署 73eba359 即修复** (针对**已有** UI key) | 调查证实 zh.json 关键 key 都已翻译, "一进去英文"是部署滞后; 真要解决的全站 fallbackLng 策略不是本面板的事 |
| 8 | ProductFlow SettingsPage 模型选择 | **不进本任务, 进 backlog (BL-2 / BL-3)** | 跨仓库改造, 涉及 ProductFlow 设计模式 (全局单模型 vs 按分组映射); 当前 task 维持 new-api 边界 |
| 9 | 详细预览面板 (Task B) | **不做, 进 backlog (BL-4)** | 用户明确拒绝, 引用原话: "渠道页面再去绑定分组 不要做的这么复杂"; 选完分组想看详情可去渠道页按分组筛选, 已有能力 |
| 10 | **清空 token_group 的能力 (r1 review P1)** | **保留**, 通过一个不与真实 group 冲突的 sentinel 选项 + onValueChange 映射回 `''` 实现 | 字段本身 Optional ([productflow-sso-settings-section.tsx:255](../../../web/default/src/features/system-settings/integrations/productflow-sso-settings-section.tsx)); 原 Input 允许空值, 不能因下拉化退化; 复用 subscriptions-mutate-drawer:260 模式, 但实现层需保证 sentinel 不与真实 group 重名, 若冲突则退化为 `null` 清空方案 |
| 11 | **本 task 新增的 UI 文案 (r1 review P2)** | **补充进 zh.json**(+4 key), en.json **不补** | R1 引入"无 token 分组"等新文案, zh.json 无对应 key 会显英文 fallback; 接受第 3 个文件改动是合理交换; en.json fallbackLng 兜底显示 key, key 本身是英文, 行为符合预期 |
| 12 | **网络失败状态 (r1 review P2)** | 用 `groupsQuery.isError` 独立分支显示错误条, 与空数据态互斥 | React Query 失败进 isError 不是 isLoading; 不区分会让 admin 把失败误认为"无分组"; 错误条比 toast 持久, 不会被忽略 |
| 13 | **自动化测试 (r1 review P2)** | **只跑 build + lint, 不新增 focused test** | SSO 面板当前**无** focused test (Glob 验证), 新增测试会破 3 文件上限; 关键状态 (sentinel/orphan/isError) 写进手工验收清单覆盖 |
| 14 | **下拉项排序稳定性 (r1 review P3)** | **前端 sort**: `default` 置顶, 其余 `localeCompare` 字典序 | `controller/group.go:16` Go map 遍历不稳定, 每次刷新顺序跳动会干扰 admin; 后端排序需改 go 文件, 前端 sort 零侵入; `default` 是 new-api 约定的"标准组", 置顶符合 admin 直觉 |

## Requirements

### R1 - Token Group 字段下拉化

**Before**:

```tsx
<FormField
  control={form.control}
  name='productflow_sso.token_group'
  render={({ field }) => (
    <FormItem>
      <FormLabel>{t('Token group')}</FormLabel>
      <FormControl>
        <Input placeholder={t('image')} {...field} ... />
      </FormControl>
      <FormDescription>
        {t('Optional New API group assigned to the token.')}
      </FormDescription>
      <FormMessage />
    </FormItem>
  )}
/>
```

**After**:

```tsx
<FormField
  control={form.control}
  name='productflow_sso.token_group'
  render={({ field }) => {
    const groupsList = groupsQuery.data ?? []
  // R3 兜底: 已存值不在列表里时,作为孤立选项显示在 sentinel 之下、普通分组之上,带 ⚠️ 提示
    const orphanCurrentValue =
      field.value && !groupsList.includes(field.value)
        ? field.value
        : null
    return (
      <FormItem>
        <FormLabel>{t('Token group')}</FormLabel>
        <FormControl>
          <Select
            value={field.value || '__none__'}
            onValueChange={(v) =>
              field.onChange(v === '__none__' ? '' : v)
            }
            disabled={groupsQuery.isLoading}
          >
            <SelectTrigger>
              <SelectValue placeholder={t('Select a group')} />
            </SelectTrigger>
            <SelectContent>
              {/* P1: 保留"清空 token_group"的能力 (字段本身 Optional, 沿用 subscriptions-mutate-drawer 模式) */}
              <SelectItem value='__none__'>{t('No token group')}</SelectItem>
              {orphanCurrentValue && (
                <SelectItem
                  value={orphanCurrentValue}
                  className='text-amber-700'
                >
                  ⚠️ {orphanCurrentValue} ({t('not in current group list')})
                </SelectItem>
              )}
              {groupsList.map((name) => (
                <SelectItem key={name} value={name}>{name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormControl>
        {/* P2: 网络失败用 isError 显式提示, 不再让 admin 误以为"无分组" */}
        {groupsQuery.isError && (
          <p className='text-xs text-destructive'>
            {t('Failed to load groups, please retry.')}
          </p>
        )}
        {/* P2: 空数据态独立分支 (isError 已先消费, 此处只可能是真空) */}
        {!groupsQuery.isLoading &&
          !groupsQuery.isError &&
          groupsList.length === 0 && (
            <p className='text-xs text-muted-foreground'>
              {t(
                'No groups available. Create one in System Settings → Models → Group Ratio.',
              )}
            </p>
          )}
        <FormDescription>
          {t('Optional New API group assigned to the token.')}
        </FormDescription>
        <FormMessage />
      </FormItem>
    )
  }}
/>
```

**P1 / P2 关键修复**:

- `__none__` sentinel 选项 `t('No token group')` —— 保留"清空 token_group"的能力, 不退化于原 Input
- `groupsQuery.isError` 独立分支 —— React Query 失败时显示错误条而非"无分组"假象
- 空数据态文字仅在**真空** (无 error 且 data.length=0) 时显示, 跟 isError 状态互斥

### R2 - 新增 `useChannelGroups` hook

在 `productflow-sso-api.ts` 中新增一个 React Query hook:

```ts
export function useChannelGroups() {
  return useQuery<string[]>({
    queryKey: ['channel-groups'],
    queryFn: async () => {
      const res = await api.get('/api/group/')
      const raw = Array.isArray(res.data?.data) ? res.data.data : []
      // P3: controller/group.go:16 是 Go map 遍历(顺序不稳定).
      // 前端 sort 保证 admin 每次打开下拉看到的顺序一致.
      // `default` 是 new-api 内建分组,固定置顶;其余按本地化字典序排.
      return [...raw].sort((a, b) => {
        if (a === 'default') return -1
        if (b === 'default') return 1
        return a.localeCompare(b)
      })
    },
    staleTime: 60_000,
  })
}
```

不带分页, 不带过滤——分组总数不会超过 50, 一次性返回完全 OK.

### R3 - 现有值兜底显示

R1 实现里 `orphanCurrentValue` 已经处理这种情况:

- 数据库已存的 `token_group` 不在 `GET /api/group/` 返回列表里 → 仍作为孤立选项在
  下拉**最上方**显示, 配 ⚠️ 标记 + `t('not in current group list')` 副文本
- admin 改选其他分组后, 孤立项不再出现 (因为 field.value 变了)
- 这是唯一一个"防呆"性质的视觉提示, 因为它针对**已经发生的事实** (而非 Decision 3
  拒绝的"预防性防呆")

### R4 - 严格控制范围 (避免任务蠕变)

**不**做以下任何一项:

- ❌ 不动 `controller/group.go` (后端零改动)
- ❌ 不升级 `GET /api/group/` 接口形态 (保持 `string[]`,users/subscriptions 消费者零回归)
- ❌ 不显示 channel_count / ratio / description
- ❌ 不加预览面板
- ❌ 不加孤儿分组扫描
- ❌ 不修复全站 SSO 面板 i18n 部署滞后问题 (BL-6, 跟本任务无关)
- ❌ 不动 ProductFlow 仓库任何文件
- ❌ 不新增 focused component test (3 文件已用满, 依赖 build + lint + 手工验收)

**允许且必须做**:

- ✅ 在 `zh.json` 补充本 task 新引入 UI 文案的翻译 key:
  - `"No token group"` → `"无 token 分组"`
  - `"Failed to load groups, please retry."` → `"分组列表加载失败，请重试。"`
  - `"No groups available. Create one in System Settings → Models → Group Ratio."` →
    `"暂无可用分组，请在 系统设置 → 模型与倍率 → 分组倍率 中创建。"`
  - `"not in current group list"` → `"不在当前分组列表"`

  其他文案 (`Select a group`, `Token group`, `Optional New API group assigned to the token.`)
  复用 zh.json 已有翻译, 不重复添加.

- ✅ `en.json` **不**需要补——i18n config 里 `fallbackLng: 'en'`, 当 en.json 缺
  key 时 i18next 显示 key 字符串本身, 而 key 已经是英文, 行为符合预期.

## Files

| 文件 | 改动类型 | 大致行数 |
|------|---------|---------|
| [productflow-sso-api.ts](../../../web/default/src/features/system-settings/integrations/productflow-sso-api.ts) | 新增 `useChannelGroups` hook (含 P3 排序) | +15 |
| [productflow-sso-settings-section.tsx](../../../web/default/src/features/system-settings/integrations/productflow-sso-settings-section.tsx) | `Input` → `Select`, 引入 hook, sentinel + orphan + isError 三态处理 | ~60 净改动 |
| [i18n/locales/zh.json](../../../web/default/src/i18n/locales/zh.json) | +4 个 key (R4 列出的中文翻译) | +4 |

总计 **3 文件**, 严格符合"每任务 ≤3 文件"全局规则.

## Out of Scope (Backlog)

下列项目都是 grill 过程中讨论后**主动延后**的, 在本 task 完成后单独立项. 每条都标记
出 grill 决策矩阵的对应条目, 方便后续接手者反向追溯.

| ID | 标题 | 触发的决策点 |
|----|------|------------|
| BL-1 | 孤儿分组健康检查 (channel.Group 贴了但 GroupRatio 没定义) | Decision 6 |
| BL-2 | ProductFlow SettingsPage 模型字段动态化 (Input → Select, 拉 `/v1/models`) | Decision 8 |
| BL-3 | ProductFlow 多分组模型映射重构 (image_generate_model → per-group) | Decision 8 |
| BL-4 | SSO Token Group 详细预览面板 (渠道+模型列表 Card) | Decision 9 |
| BL-5 | 分组删除 / 改名级联校验 (admin 删了分组但 SSO 还在引用) | Decision 隐含 (Q8 拍 A) |
| BL-6 | new-api 全站默认语言策略调整 (fallbackLng / lng) | Decision 7 衍生 (如重新部署后 SSO 仍是英文才触发) |

## Validation

### 手工验收 (admin 视角)

- [ ] 打开 SSO 配置面板, **Token group** 字段是下拉而非输入框
- [ ] 下拉打开能看到 GroupRatio 里定义的所有分组,**`default` 在最上方**, 其余按字典序
- [ ] 下拉**第一项**是"无 token 分组" (sentinel 方案), 选中后保存,
      `productflow_sso.token_group` option 写入空字符串 `''`
- [ ] 选择一个普通分组(如 `vip`)并保存, 数据库正确写入 `vip`
- [ ] 数据库里事先写一个不存在的分组名 (比如 `wrongname`), 重新打开面板,
      下拉里能看到 `wrongname` 标 ⚠️ 出现在"无 token 分组"之下、普通分组之上;
      改选别的分组并保存后, `wrongname` 不再出现
- [ ] 删除 GroupRatio 里所有非 default 分组,然后用 admin 删除 default(罕见极端),
      重进面板,下拉除 sentinel 外显示**空数据态文字**"暂无可用分组..."
- [ ] **网络失败仿真**: 临时把 `/api/group/` 路由改成 500, 重进面板,
      下拉**仍可展开**(因 sentinel 始终存在), 但下方显示**红字错误**
      "分组列表加载失败,请重试。"——**不会**显示"暂无可用分组"误导

### 自动化检查

- [ ] `bun run build` 通过 (TypeScript 类型检查)
- [ ] `bun run lint` 通过
- [ ] **不新增** focused component test, R3 / sentinel / isError 三态靠手工验收覆盖
      (3 文件已用满, 见 R4 取舍)

### 回归测试

- [ ] users 模块的分组筛选仍工作 (`features/users/api.ts:136` 调 `/api/group/`)
- [ ] subscriptions 模块的分组选择仍工作 (`features/subscriptions/api.ts:168`)
- [ ] 这两个回归点存在是因为 R4 明确"不动后端 /api/group/ 接口形态"——只要后端零改,
      它们必然不破

### i18n 验收

- [ ] 切到 zh locale 后, R4 列出的 4 个新 key 全部正确显示**中文**翻译
- [ ] 切到 en locale, 新 key 显示英文 key 字符串本身 (fallbackLng 兜底, 符合预期)

## Risks

| 风险 | 概率 | 缓解 |
|------|------|------|
| Select 组件在某些浏览器渲染问题 | 低 | 仓库内 subscriptions-mutate-drawer 等多个面板已用同款 Select, 风格一致 |
| GroupRatio 里有特殊字符的分组名 (空格 / 引号) 导致 `<SelectItem value={...}>` 异常 | 低 | new-api 的分组名格式约束在分组倍率创建时已校验; 兜底用 trim |
| admin 升级到本版本但之前 token_group 值在 GroupRatio 中不存在 | 中 | R3 已设计兜底——孤立选项 ⚠️ 显示当前值 |
| sentinel 字面值跟某个 admin 真实创建的分组名冲突 | 极低 | 实现层需在加载 group list 时检测冲突; 一旦发现同名, 退化为 `null` 清空方案或换用其他保留值, 不要把真实 group 误当成清空 |
| React Query 默认重试 3 次, 配合后端 500 会让 loading 卡 ~6 秒 | 低 | 默认行为可接受, 不在本任务调整 retry 配置; 失败后 isError 仍然正确呈现 |
