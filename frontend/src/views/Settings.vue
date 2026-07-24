<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- Page Header -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-baseline sm:justify-between">
        <div class="flex items-baseline space-x-3">
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">设置</h1>
          <span class="text-[12px] text-body-muted transition-all duration-300 select-none"
            :class="showFeedback ? 'opacity-100 translate-x-0' : 'opacity-0 -translate-x-1 pointer-events-none'">
            {{ feedbackText }}
          </span>
        </div>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-3 md:px-5 lg:px-6 pb-24">
      <!-- Configuration Cards stack -->
      <div class="space-y-6">

        <!-- Card 1: Backup Toggle & Expiry Hours -->
        <div
          class="apple-card rounded-lg p-5 sm:p-6 flex flex-col gap-4 hover:border-primary/30 hover:shadow-[0_12px_28px_rgba(0,0,0,0.02)] transition-all duration-300 bg-white">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0 flex-1">
              <div class="text-[16px] font-bold text-slate-800 tracking-tight leading-snug">保留旧版容器备份</div>
              <div class="text-[13px] text-body-muted mt-1.5 leading-relaxed">升级成功后在保留期内保存旧容器实例。若新版本运行出现问题，支持随时手动一键回滚。</div>
            </div>
            <div class="shrink-0 pt-0.5">
              <n-switch v-model:value="settings.backup_enabled" @update:value="autoSaveSettings" />
            </div>
          </div>

          <!-- Expiry hours sub-config row inside same card -->
          <div v-if="settings.backup_enabled"
            class="border-t border-hairline pt-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between transition-all duration-300">
            <div class="min-w-0 flex-1">
              <div class="text-[14px] font-bold text-slate-700 leading-snug">备份自动清除周期</div>
              <div class="text-[12px] text-body-muted mt-1 leading-relaxed">超出保留期后，系统会自动物理清除旧容器备份以释放磁盘空间。</div>
            </div>
            <div class="w-full sm:w-45 shrink-0">
              <n-select v-model:value="settings.backup_hours" :options="hoursOptions"
                @update:value="autoSaveSettings" />
            </div>
          </div>
        </div>

        <!-- Card 3: Sibling Restart Toggle -->
        <div
          class="apple-card rounded-lg p-5 sm:p-6 flex items-start justify-between gap-4 hover:border-primary/30 hover:shadow-[0_12px_28px_rgba(0,0,0,0.02)] transition-all duration-300 bg-white">
          <div class="min-w-0 flex-1">
            <div class="text-[16px] font-bold text-slate-800 tracking-tight leading-snug">自动重启同 Compose 项目服务</div>
            <div class="text-[13px] text-body-muted mt-1.5 leading-relaxed">当服务更新重建后，自动重启同 Compose 项目下的其它关联服务。</div>
          </div>
          <div class="shrink-0 pt-0.5">
            <n-switch v-model:value="settings.restart_stack" @update:value="autoSaveSettings" />
          </div>
        </div>

        <!-- Card 2: Auto Check & Update Config -->
        <div
          class="apple-card rounded-lg p-5 sm:p-6 flex flex-col gap-4 hover:border-primary/30 hover:shadow-[0_12px_28px_rgba(0,0,0,0.02)] transition-all duration-300 bg-white">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0 flex-1">
              <div class="text-[16px] font-bold text-slate-800 tracking-tight leading-snug">镜像更新检测频率</div>
              <div class="text-[13px] text-body-muted mt-1.5 leading-relaxed">自定义后台自动检测容器镜像新版本的执行间隔频率。</div>
            </div>
            <div class="flex items-center gap-2 shrink-0 mt-3 sm:mt-0">
              <n-input-number v-model:value="settings.check_value" :min="1" @update:value="autoSaveSettings" />
              <n-select v-model:value="settings.check_type" :options="checkTypeOptions"
                style="width: 82px; min-width: 82px" @update:value="autoSaveSettings" />
            </div>
          </div>

          <div
            class="border-t border-hairline pt-4 flex items-start justify-between gap-4">
            <div class="min-w-0 flex-1">
              <div class="text-[14px] font-bold text-slate-700 leading-snug">自动更新容器</div>
              <div class="text-[12px] text-body-muted mt-1 leading-relaxed">每次定时检测发现新版本时，将自动为容器执行克隆升级。</div>
            </div>
            <div class="shrink-0 pt-0.5">
              <n-switch v-model:value="settings.auto_update_enabled" @update:value="autoSaveSettings" />
            </div>
          </div>
        </div>

        <!-- Card 3.7: Notification Settings -->
        <div
          class="apple-card rounded-lg p-5 sm:p-6 hover:border-primary/30 hover:shadow-[0_12px_28px_rgba(0,0,0,0.02)] transition-all duration-300 bg-white flex flex-col gap-4">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0 flex-1">
              <div class="text-[16px] font-bold text-slate-800 tracking-tight leading-snug">通知服务</div>
              <div class="text-[13px] text-body-muted mt-1.5 leading-relaxed">在后台静默检测到更新或自动更新容器操作完成时发送通知报告。</div>
            </div>
            <div class="shrink-0 pt-0.5">
              <n-switch v-model:value="settings.notify_enabled" @update:value="autoSaveSettings" />
            </div>
          </div>

          <!-- Notification Config Form -->
          <div v-if="settings.notify_enabled"
            class="mt-2 flex flex-col gap-4 border-t border-hairline pt-4">
            
            <!-- 通知方式选择滑块 -->
            <div>
              <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-2">通知类型</label>
              <n-radio-group v-model:value="settings.notify_type" name="notify_type" size="medium" @update:value="autoSaveSettings">
                <n-radio-button value="email">邮件通知 (SMTP)</n-radio-button>
                <n-radio-button value="webhook">Webhook 推送</n-radio-button>
              </n-radio-group>
            </div>

            <!-- Email (SMTP) 设置项 -->
            <div v-if="settings.notify_type === 'email'" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div class="sm:col-span-2">
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">邮件服务商</label>
                <n-select v-model:value="smtpProvider" :options="providerOptions" @update:value="onProviderChange" />
              </div>
              <div>
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">SMTP 服务器</label>
                <n-input v-model:value="settings.smtp_host" placeholder="例如: smtp.qq.com" class="rounded-lg"
                  :disabled="smtpProvider !== 'custom'" @blur="autoSaveSettings" />
              </div>
              <div>
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">SMTP 端口</label>
                <n-input v-model:value="settings.smtp_port" placeholder="例如: 465 或 587" class="rounded-lg"
                  :disabled="smtpProvider !== 'custom'" @blur="autoSaveSettings" />
              </div>
              <div>
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">发件人账号 / 邮箱</label>
                <n-input v-model:value="settings.smtp_username" placeholder="请输入发件邮箱账号" class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div>
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">授权码 / 密码</label>
                <n-input v-model:value="settings.smtp_password" type="password" show-password-on="click"
                  placeholder="请输入授权码或密码" class="rounded-lg" @blur="autoSaveSettings" />
              </div>
              <div>
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">接收者邮箱</label>
                <n-input v-model:value="settings.smtp_to" placeholder="请输入接收报告的邮箱地址" class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div v-if="settings.auto_update_enabled">
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">自定义邮件主题模板</label>
                <n-input v-model:value="settings.smtp_subject_template"
                  placeholder="例如: [Docker Updater] 容器 {container_name} {action_type} {status}" class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div v-if="settings.auto_update_enabled" class="sm:col-span-2">
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">自定义邮件正文模板</label>
                <n-input v-model:value="settings.smtp_body_template" type="textarea"
                  :autosize="{ minRows: 4, maxRows: 10 }" placeholder="请输入邮件正文模板内容..." class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div v-if="!settings.auto_update_enabled">
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">更新提醒邮件主题模板</label>
                <n-input v-model:value="settings.smtp_subject_template_check"
                  placeholder="例如: [Docker Updater] 发现新版本: {container_name}" class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div v-if="!settings.auto_update_enabled" class="sm:col-span-2">
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">更新提醒邮件正文模板</label>
                <n-input v-model:value="settings.smtp_body_template_check" type="textarea"
                  :autosize="{ minRows: 4, maxRows: 10 }" placeholder="请输入邮件正文模板内容..." class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div class="sm:col-span-2">
                <div class="text-[11px] text-body-muted mt-2 leading-relaxed">
                  支持以下占位变量进行自动匹配：<br />
                  <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{container_name}</code>（容器名）、
                  <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{action_type}</code>（容器升级/回滚恢复/可用版本更新提醒 等）、
                  <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{status}</code>（执行成功/执行失败/发现新版本 等）、
                  <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{time}</code>（执行时间）、
                  <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{logs}</code>（运行日志/更新详情）。
                </div>
              </div>
            </div>

            <!-- Webhook 设置项 -->
            <div v-if="settings.notify_type === 'webhook'" class="grid grid-cols-1 gap-4">
              <div class="grid grid-cols-2 sm:grid-cols-5 gap-4">
                <div class="col-span-2 sm:col-span-1">
                  <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">Webhook 平台</label>
                  <n-select 
                    v-model:value="settings.webhook_type"
                    placeholder="选择推送平台" 
                    :options="[
                      { label: '自定义 Payload', value: 'custom' },
                      { label: '飞书群机器人', value: 'feishu' },
                      { label: '企业微信群机器人', value: 'wechat' },
                      { label: '钉钉群机器人', value: 'dingtalk' }
                    ]"
                    @update:value="autoSaveSettings"
                  />
                </div>
                <div class="col-span-2 sm:col-span-2">
                  <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">Webhook URL</label>
                  <n-input v-model:value="settings.webhook_url" placeholder="https://api.example.com/notify" class="rounded-lg"
                    @blur="autoSaveSettings" />
                </div>
                <div class="col-span-1 sm:col-span-1">
                  <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">请求方法</label>
                  <n-select v-model:value="settings.webhook_method" :options="[
                    { label: 'POST', value: 'POST' },
                    { label: 'GET', value: 'GET' },
                    { label: 'PUT', value: 'PUT' }
                  ]" @update:value="autoSaveSettings" />
                </div>
                <div class="col-span-1 sm:col-span-1 flex items-end">
                  <n-button type="primary" secondary round class="active-scale w-full font-semibold"
                    :loading="testingEmail" @click="sendTestNotification">
                    测试推送
                  </n-button>
                </div>
              </div>
              <div v-if="settings.auto_update_enabled">
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">{{ webhookTemplateTitle }}</label>
                <n-input v-model:value="settings['webhook_template_' + settings.webhook_type]" type="textarea"
                  :autosize="{ minRows: 4, maxRows: 10 }" placeholder="请输入 JSON 格式的通知 Payload..." class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div v-if="!settings.auto_update_enabled">
                <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">{{ webhookTemplateCheckTitle }}</label>
                <n-input v-model:value="settings['webhook_template_check_' + settings.webhook_type]" type="textarea"
                  :autosize="{ minRows: 4, maxRows: 10 }" placeholder="请输入 JSON 格式的通知 Payload..." class="rounded-lg"
                  @blur="autoSaveSettings" />
              </div>
              <div class="text-[11px] text-body-muted mt-2 leading-relaxed">
                <div v-if="settings.webhook_type === 'custom'" class="text-slate-500 font-medium mb-1">
                  提示：自定义模式下，请根据您的 Webhook 提供商提供的参数格式进行配置。
                </div>
                支持以下占位变量进行自动匹配：<br />
                <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{container_name}</code>（容器名）、
                <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{action_type}</code>（容器升级/回滚恢复/可用版本更新提醒 等）、
                <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{status}</code>（执行成功/执行失败/发现新版本 等）、
                <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{time}</code>（执行时间）、
                <code class="text-primary font-mono font-bold bg-slate-50 border border-hairline px-1 rounded">{logs}</code>（运行日志/更新详情，换行符会被自动转义）。
              </div>
            </div>

            <!-- 发送测试按钮部分 (仅在 Email 模式下显示在底部) -->
            <div v-if="settings.notify_type === 'email'" class="flex items-center justify-between sm:justify-start gap-6 mt-5">
              <div class="flex items-center gap-2">
                <span class="text-[13px] text-slate-600 font-medium">启用 SSL 加密</span>
                <n-switch v-model:value="settings.smtp_ssl" :disabled="smtpProvider !== 'custom'"
                  @update:value="autoSaveSettings" />
              </div>
              <n-button secondary round size="small"
                class="active-scale bg-surface-pearl border border-divider-soft text-slate-700 font-semibold"
                :loading="testingEmail" @click="sendTestNotification">
                测试推送
              </n-button>
            </div>
          </div>
        </div>


        <!-- Card 4: Private Registry Credentials Management -->
        <div
          class="apple-card rounded-lg p-5 sm:p-6 hover:border-primary/30 hover:shadow-[0_12px_28px_rgba(0,0,0,0.02)] transition-all duration-300 bg-white flex flex-col gap-5">
          <!-- Card Header (Title & Button in one row) -->
          <div class="flex flex-col gap-1.5">
            <div class="flex items-center justify-between gap-4">
              <div class="text-[16px] font-bold text-slate-800 tracking-tight leading-snug">仓库凭证</div>
              <n-button secondary circle size="small"
                class="active-scale bg-surface-pearl border border-divider-soft text-slate-700 font-semibold shrink-0"
                title="添加仓库凭证"
                @click="openAddRegistryModal">
                <template #icon>
                  <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
                    stroke-linecap="round" stroke-linejoin="round">
                    <line x1="12" y1="5" x2="12" y2="19"></line>
                    <line x1="5" y1="12" x2="19" y2="12"></line>
                  </svg>
                </template>
              </n-button>
            </div>
            <div class="text-[13px] text-body-muted leading-relaxed">配置镜像源仓库（如阿里云、自建 Harbor 等）的认证账号，用于拉取和比对私有镜像。</div>
          </div>

          <!-- Credentials List -->
          <div>
            <div v-if="registries.length > 0" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              <div v-for="r in registries" :key="r.id"
                class="flex items-center justify-between p-3.5 bg-slate-50/40 hover:bg-slate-50 border border-hairline rounded-xl transition-all duration-200">
                <div class="min-w-0 flex-1 pr-3">
                  <div class="text-[14px] font-semibold text-slate-800 break-all leading-tight" :title="r.registry">{{
                    r.registry }}</div>
                  <div class="text-[12px] text-body-muted mt-1.5 font-mono">用户名: <span
                      class="text-slate-600 font-sans font-medium">{{ r.username }}</span></div>
                </div>
                <div class="flex items-center gap-2 shrink-0">
                  <n-button size="tiny" round class="active-scale text-[11px] font-medium" @click="editRegistry(r)">
                    编辑
                  </n-button>
                  <n-button size="tiny" type="error" ghost round class="active-scale text-[11px] font-medium"
                    @click="deleteRegistry(r.id)">
                    删除
                  </n-button>
                </div>
              </div>
            </div>

            <div v-else
              class="text-center py-8 text-body-muted text-[13px] bg-slate-50/20 rounded-xl border border-dashed border-hairline select-none flex flex-col items-center justify-center">
              暂未配置任何仓库凭证
            </div>
          </div>
        </div>

        <!-- Card 4: Registry Mirrors (Temporary) -->
        <div
          class="apple-card rounded-lg p-5 sm:p-6 hover:border-primary/30 hover:shadow-[0_12px_28px_rgba(0,0,0,0.02)] transition-all duration-300 bg-white flex flex-col gap-5">
          <!-- Card Header (Title & Button in one row) -->
          <div class="flex flex-col gap-1.5">
            <div class="flex items-center justify-between gap-4">
              <div class="text-[16px] font-bold text-slate-800 tracking-tight leading-snug">镜像加速源</div>
              <n-button secondary circle size="small"
                class="active-scale bg-surface-pearl border border-divider-soft text-slate-700 font-semibold shrink-0"
                title="添加镜像加速源"
                @click="mirrorModalVisible = true">
                <template #icon>
                  <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
                    stroke-linecap="round" stroke-linejoin="round">
                    <line x1="12" y1="5" x2="12" y2="19"></line>
                    <line x1="5" y1="12" x2="19" y2="12"></line>
                  </svg>
                </template>
              </n-button>
            </div>
            <div class="text-[13px] text-body-muted leading-relaxed">配置当前应用的 Docker 镜像加速源。</div>
          </div>

          <div class="flex flex-col gap-3">
            <!-- Temporary Mirrors List -->
            <div v-if="settings.temp_mirrors && settings.temp_mirrors.length > 0"
              class="border border-hairline rounded-xl overflow-hidden divide-y divide-hairline bg-slate-50/5">
              <div v-for="(m, idx) in settings.temp_mirrors" :key="idx"
                class="flex items-center justify-between p-3.5">
                <span class="text-[13px] font-mono text-slate-700 break-all pr-4">{{ m }}</span>
                <n-button size="tiny" type="error" ghost round class="active-scale shrink-0"
                  @click="removeTempMirror(idx)">
                  删除
                </n-button>
              </div>
            </div>
            <div v-else
              class="text-center py-6 text-body-muted text-[12px] bg-slate-50/20 rounded-xl border border-dashed border-hairline select-none flex flex-col items-center justify-center">
              暂未配置任何镜像加速源
            </div>
          </div>
        </div>

      </div>
    </div>

    <!-- Registry Credentials Modal -->
    <n-modal v-model:show="registryModalVisible" transform-origin="center">
      <div class="w-[90vw] max-w-115 bg-white rounded-lg border border-hairline p-6 shadow-xl relative select-none">
        <div class="text-[18px] font-bold text-slate-800 tracking-tight mb-4">
          {{ editingRegistryId ? '编辑仓库凭据' : '添加仓库凭据' }}
        </div>

        <div class="space-y-4 py-2">
          <div>
            <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">仓库域名</label>
            <n-input v-model:value="registryForm.registry" placeholder="例如: registry.cn-hangzhou.aliyuncs.com"
              class="rounded-lg" />
          </div>
          <div>
            <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">用户名</label>
            <n-input v-model:value="registryForm.username" placeholder="请输入用户名" class="rounded-lg" />
          </div>
          <div>
            <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">密码 / Token</label>
            <n-input v-model:value="registryForm.password" type="password" show-password-on="click"
              placeholder="请输入密码或 Token" class="rounded-lg" />
          </div>
        </div>

        <div class="flex items-center justify-end gap-3 mt-6">
          <n-button round class="active-scale px-5" @click="registryModalVisible = false">
            取消
          </n-button>
          <n-button type="primary" round class="active-scale px-5 font-semibold" :loading="submittingRegistry"
            @click="submitRegistry">
            保存凭据
          </n-button>
        </div>
      </div>
    </n-modal>

    <!-- Add Mirror Modal -->
    <n-modal v-model:show="mirrorModalVisible" transform-origin="center">
      <div class="w-[90vw] max-w-115 bg-white rounded-lg border border-hairline p-6 shadow-xl relative select-none">
        <div class="text-[18px] font-bold text-slate-800 tracking-tight mb-4">
          添加镜像加速源
        </div>

        <div class="space-y-4 py-2">
          <div>
            <label class="text-[12px] font-semibold tracking-wider text-slate-500 block mb-1.5">加速源地址</label>
            <n-input v-model:value="newTempMirror" placeholder="例如: https://docker.m.daocloud.io"
              class="rounded-lg" @keyup.enter="addTempMirror" />
          </div>

          <div class="pt-1.5">
            <div class="text-[12px] font-semibold tracking-wider text-slate-500 mb-2">常用镜像源</div>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="url in recommendedMirrors"
                :key="url"
                class="px-2.5 py-1 text-[11px] font-mono rounded-lg border border-slate-200 hover:border-primary hover:text-primary bg-slate-50/50 hover:bg-primary-50/20 active:scale-95 transition-all cursor-pointer select-none"
                @click="newTempMirror = url"
              >
                {{ url.replace('https://', '') }}
              </button>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-3 mt-6">
          <n-button round class="active-scale px-5" @click="mirrorModalVisible = false">
            取消
          </n-button>
          <n-button type="primary" round class="active-scale px-5 font-semibold"
            @click="addTempMirror">
            确定
          </n-button>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import { NButton, NSwitch, NSelect, NInput, NInputNumber, NModal, useMessage, useDialog, NRadioGroup, NRadioButton } from 'naive-ui'
import axios from 'axios'

const apiBase = '/app/docker-updater/api'
const message = useMessage()
const dialog = useDialog()

interface SettingsData {
  backup_enabled: boolean;
  backup_hours: number;
  restart_stack: boolean;
  auto_update_enabled: boolean;
  temp_mirrors: string[];
  check_type: string;
  check_value: number;
  notify_enabled: boolean;
  notify_type: string;
  smtp_enabled: boolean;
  smtp_host: string;
  smtp_port: string;
  smtp_username: string;
  smtp_password: string;
  smtp_ssl: boolean;
  smtp_to: string;
  smtp_subject_template: string;
  smtp_body_template: string;
  smtp_subject_template_check: string;
  smtp_body_template_check: string;
  webhook_url: string;
  webhook_method: string;
  webhook_type: string;
  webhook_template_custom: string;
  webhook_template_wechat: string;
  webhook_template_dingtalk: string;
  webhook_template_feishu: string;
  webhook_template_check_custom: string;
  webhook_template_check_wechat: string;
  webhook_template_check_dingtalk: string;
  webhook_template_check_feishu: string;
  [key: string]: any;
}

const settings = ref<SettingsData>({
  backup_enabled: false,
  backup_hours: 24,
  restart_stack: false,
  auto_update_enabled: false,
  temp_mirrors: [],
  check_type: 'day',
  check_value: 1,
  notify_enabled: false,
  notify_type: 'email',
  smtp_enabled: false,
  smtp_host: '',
  smtp_port: '465',
  smtp_username: '',
  smtp_password: '',
  smtp_ssl: true,
  smtp_to: '',
  smtp_subject_template: '',
  smtp_body_template: '',
  smtp_subject_template_check: '',
  smtp_body_template_check: '',
  webhook_url: '',
  webhook_method: 'POST',
  webhook_type: 'custom',
  webhook_template_custom: '',
  webhook_template_wechat: '',
  webhook_template_dingtalk: '',
  webhook_template_feishu: '',
  webhook_template_check_custom: '',
  webhook_template_check_wechat: '',
  webhook_template_check_dingtalk: '',
  webhook_template_check_feishu: ''
})

const webhookTemplateTitle = computed(() => {
  switch (settings.value.webhook_type) {
    case 'feishu':
      return '飞书群机器人通知模板'
    case 'wechat':
      return '企业微信群机器人通知模板'
    case 'dingtalk':
      return '钉钉群机器人通知模板'
    default:
      return '自定义推送服务模板'
  }
})

const webhookTemplateCheckTitle = computed(() => {
  switch (settings.value.webhook_type) {
    case 'feishu':
      return '飞书群机器人更新提醒模板'
    case 'wechat':
      return '企业微信群机器人更新提醒模板'
    case 'dingtalk':
      return '钉钉群机器人更新提醒模板'
    default:
      return '自定义推送服务更新提醒模板'
  }
})

const registries = ref<any[]>([])
const registryModalVisible = ref<boolean>(false)
const submittingRegistry = ref<boolean>(false)
const editingRegistryId = ref<number | null>(null)
const registryForm = ref({
  registry: '',
  username: '',
  password: ''
})

const newTempMirror = ref<string>('')
const mirrorModalVisible = ref<boolean>(false)

const recommendedMirrors = [
  'https://docker.1ms.run',
  'https://docker.m.daocloud.io',
  'https://atomhub.openatom.cn',
  'https://docker.fxxk.dedyn.io'
]

const saving = ref<boolean>(false)
const showFeedback = ref<boolean>(false)
const feedbackText = ref<string>('')
let feedbackTimer: any = null
let autoSaveTimer: any = null

const hoursOptions = [
  { label: '保留 12 小时', value: 12 },
  { label: '保留 1 天', value: 24 },
  { label: '保留 3 天', value: 72 },
  { label: '保留 7 天', value: 168 },
  { label: '保留 1 个月', value: 720 }
]

const checkTypeOptions = [
  { label: '时', value: 'hour' },
  { label: '天', value: 'day' },
  { label: '周', value: 'week' },
  { label: '月', value: 'month' }
]

const testingEmail = ref<boolean>(false)

const smtpProvider = ref<string>('custom')

const providerOptions = [
  { label: '自定义', value: 'custom' },
  { label: 'QQ 邮箱', value: 'qq' },
  { label: '163 网易邮箱', value: '163' },
  { label: 'Gmail', value: 'gmail' },
  { label: 'Outlook', value: 'outlook' }
]

const providerPresets: Record<string, { host: string; port: string; ssl: boolean }> = {
  qq: { host: 'smtp.qq.com', port: '465', ssl: true },
  '163': { host: 'smtp.163.com', port: '465', ssl: true },
  gmail: { host: 'smtp.gmail.com', port: '465', ssl: true },
  outlook: { host: 'smtp.office365.com', port: '587', ssl: false }
}

const onProviderChange = (val: string) => {
  if (val !== 'custom') {
    const preset = providerPresets[val]
    if (preset) {
      settings.value.smtp_host = preset.host
      settings.value.smtp_port = preset.port
      settings.value.smtp_ssl = preset.ssl
      autoSaveSettings()
    }
  }
}

const detectProvider = () => {
  const host = settings.value.smtp_host
  const port = settings.value.smtp_port
  const ssl = settings.value.smtp_ssl

  for (const [key, preset] of Object.entries(providerPresets)) {
    if (preset.host === host && preset.port === port && preset.ssl === ssl) {
      smtpProvider.value = key
      return
    }
  }
  smtpProvider.value = 'custom'
}

const sendTestNotification = async () => {
  if (settings.value.notify_type === 'email') {
    if (!settings.value.smtp_host || !settings.value.smtp_username || !settings.value.smtp_password || !settings.value.smtp_to) {
      message.warning('请先填写完整的邮件通知配置（SMTP 服务器、账号、授权码和收件人）')
      return
    }
  } else if (settings.value.notify_type === 'webhook') {
    if (!settings.value.webhook_url) {
      message.warning('请先填写 Webhook URL')
      return
    }
  }
  testingEmail.value = true
  try {
    const res = await axios.post(`${apiBase}/settings/test-email`, {
      notify_type: settings.value.notify_type,
      smtp_host: settings.value.smtp_host,
      smtp_port: settings.value.smtp_port,
      smtp_username: settings.value.smtp_username,
      smtp_password: settings.value.smtp_password,
      smtp_ssl: settings.value.smtp_ssl,
      smtp_to: settings.value.smtp_to,
      smtp_subject_template: settings.value.auto_update_enabled ? settings.value.smtp_subject_template : settings.value.smtp_subject_template_check,
      smtp_body_template: settings.value.auto_update_enabled ? settings.value.smtp_body_template : settings.value.smtp_body_template_check,
      webhook_url: settings.value.webhook_url,
      webhook_method: settings.value.webhook_method,
      webhook_type: settings.value.webhook_type,
      webhook_template_custom: settings.value.auto_update_enabled ? settings.value.webhook_template_custom : settings.value.webhook_template_check_custom,
      webhook_template_wechat: settings.value.auto_update_enabled ? settings.value.webhook_template_wechat : settings.value.webhook_template_check_wechat,
      webhook_template_dingtalk: settings.value.auto_update_enabled ? settings.value.webhook_template_dingtalk : settings.value.webhook_template_check_dingtalk,
      webhook_template_feishu: settings.value.auto_update_enabled ? settings.value.webhook_template_feishu : settings.value.webhook_template_check_feishu
    })
    const respBody = res.data?.response
    if (settings.value.notify_type === 'email') {
      message.success('测试邮件发送成功，请前往您的收件箱查收！')
    } else {
      let displayResp = respBody || ''
      if (displayResp.length > 80) {
        displayResp = displayResp.substring(0, 80) + '...'
      }
      message.success(`测试 Webhook 发送成功！平台响应: ${displayResp || '无'}`)
    }
  } catch (err: any) {
    const errorMsg = err.response?.data?.error || err.message || '网络连接异常'
    const respBody = err.response?.data?.response
    if (respBody) {
      dialog.error({
        title: '测试发送失败',
        content: () => h('pre', { class: 'bg-slate-50/50 border border-slate-200/80 rounded-lg p-3 text-[11px] font-mono text-slate-600 whitespace-pre-wrap break-all overflow-x-auto mt-2 max-h-[220px] overflow-y-auto' }, respBody),
        positiveText: '好的'
      })
    } else {
      message.error(`测试发送失败: ${errorMsg}`)
    }
  } finally {
    testingEmail.value = false
  }
}

const loadSettings = async () => {
  try {
    const res = await axios.get(`${apiBase}/settings`)
    settings.value = res.data
    if (!settings.value.temp_mirrors) {
      settings.value.temp_mirrors = []
    }
    if (settings.value.notify_type === undefined || settings.value.notify_type === '') {
      settings.value.notify_type = 'email'
    }
    if (settings.value.webhook_method === undefined || settings.value.webhook_method === '') {
      settings.value.webhook_method = 'POST'
    }

    if (settings.value.smtp_subject_template_check === undefined) {
      settings.value.smtp_subject_template_check = ''
    }
    if (settings.value.smtp_body_template_check === undefined) {
      settings.value.smtp_body_template_check = ''
    }
    if (settings.value.webhook_type === undefined || settings.value.webhook_type === '') {
      settings.value.webhook_type = 'custom'
    }
    const tplKeys = [
      'webhook_template_custom', 'webhook_template_wechat', 'webhook_template_dingtalk', 'webhook_template_feishu',
      'webhook_template_check_custom', 'webhook_template_check_wechat', 'webhook_template_check_dingtalk', 'webhook_template_check_feishu'
    ]
    tplKeys.forEach(k => {
      if (settings.value[k] === undefined) {
        settings.value[k] = ''
      }
    })
    detectProvider()
  } catch (err) {
    message.error('载入配置失败')
  }
}

const autoSaveSettings = async () => {
  if (autoSaveTimer) {
    clearTimeout(autoSaveTimer)
  }
  if (feedbackTimer) {
    clearTimeout(feedbackTimer)
  }

  feedbackText.value = '正在同步...'
  showFeedback.value = true
  saving.value = true

  autoSaveTimer = setTimeout(async () => {
    try {
      await axios.post(`${apiBase}/settings`, settings.value)
      saving.value = false
      feedbackText.value = '配置已自动保存'

      // 2.5秒后渐变淡出
      feedbackTimer = setTimeout(() => {
        showFeedback.value = false
      }, 2500)
    } catch (err) {
      saving.value = false
      feedbackText.value = '保存配置失败'
      feedbackTimer = setTimeout(() => {
        showFeedback.value = false
      }, 3000)
    }
  }, 300)
}

const loadRegistries = async () => {
  try {
    const res = await axios.get(`${apiBase}/registries`)
    registries.value = res.data || []
  } catch (err) {
    message.error('载入私有源列表失败')
  }
}


const addTempMirror = async () => {
  const url = newTempMirror.value.trim()
  if (!url) {
    message.warning('请输入加速源地址')
    return
  }
  if (!settings.value.temp_mirrors) {
    settings.value.temp_mirrors = []
  }
  if (!settings.value.temp_mirrors.includes(url)) {
    settings.value.temp_mirrors.push(url)
    newTempMirror.value = ''
    mirrorModalVisible.value = false
    await autoSaveSettings()
    message.success('添加成功')
  } else {
    message.warning('该加速源已存在')
  }
}

const removeTempMirror = async (index: number) => {
  if (settings.value.temp_mirrors) {
    settings.value.temp_mirrors.splice(index, 1)
    await autoSaveSettings()
  }
}

const openAddRegistryModal = () => {
  editingRegistryId.value = null
  registryForm.value = { registry: '', username: '', password: '' }
  registryModalVisible.value = true
}

const editRegistry = (item: any) => {
  editingRegistryId.value = item.id
  registryForm.value = {
    registry: item.registry,
    username: item.username,
    password: item.password
  }
  registryModalVisible.value = true
}

const copyText = (text: string) => {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(() => {
      message.success('已复制到剪贴板')
    }).catch(() => {
      message.error('复制失败')
    })
  } else {
    const input = document.createElement('input')
    input.setAttribute('value', text)
    document.body.appendChild(input)
    input.select()
    try {
      document.execCommand('copy')
      message.success('已复制到剪贴板')
    } catch {
      message.error('复制失败')
    }
    document.body.removeChild(input)
  }
}

const submitRegistry = async () => {
  if (!registryForm.value.registry || !registryForm.value.username || !registryForm.value.password) {
    message.warning('请填写完整的认证信息')
    return false
  }

  submittingRegistry.value = true
  try {
    await axios.post(`${apiBase}/registries`, {
      id: editingRegistryId.value || 0,
      registry: registryForm.value.registry,
      username: registryForm.value.username,
      password: registryForm.value.password
    })
    message.success('凭证保存成功')
    loadRegistries()
    registryModalVisible.value = false
  } catch (err) {
    message.error('保存凭证失败')
    return false
  } finally {
    submittingRegistry.value = false
  }
}


const deleteRegistry = (id: number) => {
  dialog.warning({
    title: '删除 Registry 凭据',
    content: '确认删除该 Registry 镜像认证凭证吗？删除后将导致针对该镜像源的容器服务无法正常升级。',
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: () => {
      return new Promise<void>(async (resolve, reject) => {
        try {
          await axios.delete(`${apiBase}/registries/${id}`)
          message.success('凭证删除成功')
          loadRegistries()
          resolve()
        } catch (err) {
          message.error('删除凭证失败')
          reject()
        }
      })
    }
  })
}



onMounted(() => {
  loadSettings()
  loadRegistries()
})
</script>
